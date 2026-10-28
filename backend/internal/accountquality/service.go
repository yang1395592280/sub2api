package accountquality

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Service struct {
	store     *Store
	provider  Provider
	snapshot  atomic.Pointer[Snapshot]
	cancel    context.CancelFunc
	done      chan struct{}
	workers   sync.WaitGroup
	startOnce sync.Once
	stopOnce  sync.Once
}
type Snapshot struct {
	Settings Settings
	Policies []Policy
	At       time.Time
}

func NewService(store *Store, provider Provider) *Service {
	return &Service{store: store, provider: provider, done: make(chan struct{})}
}
func (s *Service) Start() {
	s.startOnce.Do(func() { ctx, cancel := context.WithCancel(context.Background()); s.cancel = cancel; go s.loop(ctx) })
}
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
			<-s.done
		}
		s.workers.Wait()
	})
}
func (s *Service) Read(ctx context.Context, f func(*Tx) (any, error)) (any, error) {
	return s.store.Transaction(ctx, true, f)
}
func (s *Service) Write(ctx context.Context, f func(*Tx) (any, error)) (any, error) {
	v, err := s.store.Transaction(ctx, false, f)
	if err == nil {
		s.Refresh(ctx)
	}
	return v, err
}
func (s *Service) Refresh(ctx context.Context) {
	v, err := s.Read(ctx, func(t *Tx) (any, error) {
		snapshot := &Snapshot{Settings: t.Settings, At: t.Now}
		if t.Settings.ControlEnabled {
			p, e := t.Policies(false)
			if e != nil {
				return nil, e
			}
			snapshot.Policies = p
		}
		return snapshot, nil
	})
	if err == nil {
		s.snapshot.Store(v.(*Snapshot))
	}
}
func (s *Service) Health() map[string]any {
	v := s.snapshot.Load()
	if v == nil {
		return map[string]any{"store_status": "unavailable", "last_sync_at": nil}
	}
	status := "healthy"
	if time.Since(v.At) > 60*time.Second {
		status = "unavailable"
	}
	return map[string]any{"store_status": status, "last_sync_at": v.At}
}

// Decision is called only for requests carrying the trusted original auto mode.
// The caller supplies a scope resolved using the real outbound model/parameters.
func (s *Service) Decision(accountID int64, controlScope string, now time.Time) (string, error) {
	v := s.snapshot.Load()
	if v == nil {
		return "", Fail(503, "QUALITY_STATE_UNAVAILABLE", "质量状态尚未加载")
	}
	if !v.Settings.ControlEnabled {
		return "baseline", nil
	}
	for _, p := range v.Policies {
		if p.AccountID != accountID || p.ControlScopeHash != controlScope {
			continue
		}
		p = Present(p, v.Settings, now)
		if !p.Control.Effective {
			continue
		}
		if p.Control.Action == "deny" {
			return "deny", nil
		}
		if now.Sub(v.At) > 60*time.Second {
			return "", Fail(503, "QUALITY_STATE_UNAVAILABLE", "质量状态已过期")
		}
		return p.Control.Action, nil
	}
	if now.Sub(v.At) > 60*time.Second {
		return "", Fail(503, "QUALITY_STATE_UNAVAILABLE", "质量状态已过期")
	}
	return "baseline", nil
}
func (s *Service) Enabled() bool {
	v := s.snapshot.Load()
	return v == nil || v.Settings.ControlEnabled
}

type CreateItem struct {
	AccountID                int64     `json:"account_id"`
	SourceGroupID            int64     `json:"source_group_id"`
	Condition                Condition `json:"condition"`
	Rules                    Rules     `json:"rules"`
	ReplacesPolicyID         int64     `json:"replaces_policy_id,omitempty"`
	ReplacesExpectedRevision int64     `json:"replaces_expected_revision,omitempty"`
}
type UpdateItem struct {
	PolicyID         int64 `json:"policy_id"`
	ExpectedRevision int64 `json:"expected_revision"`
	Rules            Rules `json:"rules"`
}

func (s *Service) Resolve(ctx context.Context, item CreateItem) (Target, error) {
	if item.AccountID <= 0 || item.SourceGroupID <= 0 {
		return Target{}, Invalid("account_id", "请选择账号及有效来源组")
	}
	if err := item.Condition.Validate(); err != nil {
		return Target{}, err
	}
	return s.resolveTarget(ctx, item.AccountID, item.SourceGroupID, item.Condition)
}
func (s *Service) Create(t *Tx, items []CreateItem, targets []Target, actor int64) (any, error) {
	if len(items) == 0 || len(items) > 100 || len(items) != len(targets) {
		return nil, Invalid("items", "请选择1–100个策略")
	}
	out := []Policy{}
	seen := map[string]bool{}
	replaced := map[int64]bool{}
	for i, item := range items {
		if err := item.Rules.Validate(); err != nil {
			return nil, err
		}
		target := targets[i]
		if !target.CanProbe || target.Eligibility != "eligible" {
			return nil, Fail(400, "QUALITY_UNSUPPORTED_CONDITION", "该条件不可检测")
		}
		if item.Rules.ControlEnabled && !target.CanEnforce {
			return nil, Fail(400, "QUALITY_UNSUPPORTED_CONDITION", "该条件暂不可控制")
		}
		p := Policy{AccountID: item.AccountID, SourceGroupID: item.SourceGroupID, Condition: item.Condition, Rules: item.Rules, RoutingMode: RoutingMode, Revision: 1}
		p.SetTarget(target)
		if seen[p.ControlScopeHash] {
			return nil, Fail(409, "QUALITY_SCOPE_CONFLICT", "批量内存在重复有效范围")
		}
		seen[p.ControlScopeHash] = true
		if item.ReplacesPolicyID > 0 {
			if replaced[item.ReplacesPolicyID] {
				return nil, Invalid("replaces_policy_id", "不能重复替换策略")
			}
			replaced[item.ReplacesPolicyID] = true
			old, err := t.Policy(item.ReplacesPolicyID, true)
			if err != nil {
				return nil, err
			}
			if old.AccountID != item.AccountID {
				return nil, Invalid("replaces_policy_id", "替换策略必须属于同一账号")
			}
			if old.Revision != item.ReplacesExpectedRevision {
				return nil, revisionConflict()
			}
			if _, err = s.Lifecycle(t, &old, "archive", "替换检测条件", actor); err != nil {
				return nil, err
			}
		}
		if p.Rules.ControlEnabled {
			p.ControlEpoch = 1
		}
		p.State = NewState(t.Settings.ControlEpoch, p.ControlEpoch)
		schedule(&p, t.Settings, t.Now, true)
		if err := t.SavePolicy(&p); err != nil {
			return nil, err
		}
		if err := t.AddEvent(&p, nil, "policy_created", "", "created", actor); err != nil {
			return nil, err
		}
		out = append(out, Present(p, t.Settings, t.Now))
	}
	return map[string]any{"items": out}, nil
}
func revisionConflict() error {
	return Fail(409, "QUALITY_REVISION_CONFLICT", "配置已被其他操作更新，请刷新")
}
func schedule(p *Policy, settings Settings, now time.Time, first bool) {
	p.NextRunAt = nil
	if p.ArchivedAt != nil || p.Eligibility != "eligible" || !settings.AutomationEnabled || !p.Rules.AutoCheckEnabled {
		return
	}
	seconds := p.Rules.IntervalSeconds
	if p.State.Isolated() {
		seconds = p.Rules.RecoveryIntervalSeconds
	}
	if first {
		seconds = 0
	}
	// Stable positive jitter avoids synchronized batches without using global RNG.
	jitter := 0
	if p.Rules.JitterSeconds > 0 {
		jitter = int((p.ID + now.Unix()) % int64(p.Rules.JitterSeconds+1))
	}
	next := now.Add(time.Duration(seconds+jitter) * time.Second)
	if p.State.Isolated() && p.State.CooldownUntil != nil && next.Before(*p.State.CooldownUntil) {
		next = *p.State.CooldownUntil
	}
	p.NextRunAt = &next
}
func (s *Service) Update(t *Tx, items []UpdateItem, actor int64) (any, error) {
	if len(items) == 0 || len(items) > 100 {
		return nil, Invalid("items", "请选择1–100个策略")
	}
	out := []Policy{}
	seen := map[int64]bool{}
	for _, item := range items {
		if item.PolicyID <= 0 || seen[item.PolicyID] {
			return nil, Invalid("items", "策略ID必须唯一")
		}
		seen[item.PolicyID] = true
		if err := item.Rules.Validate(); err != nil {
			return nil, err
		}
		p, err := t.Policy(item.PolicyID, true)
		if err != nil {
			return nil, err
		}
		if p.ArchivedAt != nil {
			return nil, Fail(409, "QUALITY_POLICY_ARCHIVED", "策略已归档")
		}
		if p.Revision != item.ExpectedRevision {
			return nil, revisionConflict()
		}
		// Resolve against the current route before enabling control. The result is
		// never used to silently broaden the saved evidence scope.
		target, err := s.resolveTarget(t.ctx, p.AccountID, p.SourceGroupID, p.Condition)
		if err != nil {
			return nil, err
		}
		candidate := p
		candidate.SetTarget(target)
		if candidate.ScopeHash != p.ScopeHash {
			return nil, Fail(409, "QUALITY_SCOPE_CONFLICT", "路由已变化，请刷新后替换策略")
		}
		if item.Rules.ControlEnabled && !target.CanEnforce {
			return nil, Fail(400, "QUALITY_UNSUPPORTED_CONDITION", "该条件暂不可控制")
		}
		old := p.Rules
		p.Rules = item.Rules
		p.Revision++
		if !old.ControlEnabled && p.Rules.ControlEnabled {
			p.ControlEpoch++
			p.State = NewState(t.Settings.ControlEpoch, p.ControlEpoch)
		}
		schedule(&p, t.Settings, t.Now, !old.AutoCheckEnabled && p.Rules.AutoCheckEnabled)
		if err = t.CancelUnsent(p.ID, false, "policy_revision_changed"); err != nil {
			return nil, err
		}
		if err = t.SavePolicy(&p); err != nil {
			return nil, err
		}
		if err = t.AddEvent(&p, nil, "policy_updated", p.State.Name, "rules_updated", actor); err != nil {
			return nil, err
		}
		out = append(out, Present(p, t.Settings, t.Now))
	}
	return map[string]any{"items": out}, nil
}
func (s *Service) Lifecycle(t *Tx, p *Policy, action, reason string, actor int64) (any, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return nil, Invalid("reason", "请填写1–500字节原因")
	}
	if p.ArchivedAt != nil {
		return nil, Fail(409, "QUALITY_POLICY_ARCHIVED", "策略已归档")
	}
	old := p.State.Name
	p.Revision++
	p.ControlEpoch++
	if action == "release" {
		if !p.State.Isolated() {
			return nil, Fail(409, "QUALITY_RELEASE_NOT_APPLICABLE", "当前策略未处于隔离或恢复中")
		}
		p.State = NewState(t.Settings.ControlEpoch, p.ControlEpoch)
		schedule(p, t.Settings, t.Now, true)
	} else if action == "archive" {
		p.ArchivedAt = &t.Now
		p.Rules.AutoCheckEnabled = false
		p.Rules.ControlEnabled = false
		p.NextRunAt = nil
	} else {
		return nil, Invalid("action", "不支持的操作")
	}
	if err := t.CancelUnsent(p.ID, false, action); err != nil {
		return nil, err
	}
	if action == "archive" {
		if _, err := t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET cancel_requested=TRUE WHERE policy_id=$1 AND status='running'`, p.ID); err != nil {
			return nil, err
		}
	}
	if err := t.SavePolicy(p); err != nil {
		return nil, err
	}
	if err := t.AddEvent(p, nil, action, old, reason, actor); err != nil {
		return nil, err
	}
	return Present(*p, t.Settings, t.Now), nil
}
func (s *Service) ChangeSettings(t *Tx, expected int64, settings Settings, actor int64, emergency bool, reason string) (any, error) {
	old := t.Settings
	if !emergency && expected != old.Revision {
		return nil, revisionConflict()
	}
	if emergency {
		if strings.TrimSpace(reason) == "" || len(reason) > 500 {
			return nil, Invalid("reason", "请填写停用原因")
		}
		settings = old
		settings.AutomationEnabled = false
		settings.ControlEnabled = false
	}
	settings.Revision = old.Revision + 1
	settings.ControlEpoch = old.ControlEpoch
	if !old.ControlEnabled && settings.ControlEnabled {
		settings.ControlEpoch++
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	if err := t.SaveSettings(settings); err != nil {
		return nil, err
	}
	if old.AutomationEnabled != settings.AutomationEnabled {
		if !settings.AutomationEnabled {
			if err := t.CancelUnsent(0, true, "automation_disabled"); err != nil {
				return nil, err
			}
		}
		policies, err := t.Policies(false)
		if err != nil {
			return nil, err
		}
		for _, p := range policies {
			p = Present(p, settings, t.Now)
			schedule(&p, settings, t.Now, true)
			if err = t.SavePolicy(&p); err != nil {
				return nil, err
			}
		}
	}
	if err := t.AddEvent(nil, nil, "settings_changed", "", reason, actor); err != nil {
		return nil, err
	}
	return map[string]any{"settings": settings, "propagation_target_seconds": 5}, nil
}
func (s *Service) Submit(t *Tx, ids []int64, actor int64) (any, error) {
	if len(ids) == 0 || len(ids) > 100 {
		return nil, Invalid("policy_ids", "请选择1–100个策略")
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return nil, Invalid("policy_ids", "ID必须为正数且唯一")
		}
		seen[id] = true
	}
	batch := newBatchID()
	items := []Submission{}
	for _, id := range ids {
		p, err := t.Policy(id, true)
		if err != nil {
			var e *Error
			if errors.As(err, &e) && e.Status == 404 {
				items = append(items, Submission{PolicyID: id, Submission: "skipped", Reason: e.Code})
				continue
			}
			return nil, err
		}
		if p.ArchivedAt != nil {
			items = append(items, Submission{PolicyID: id, Submission: "skipped", Reason: "QUALITY_POLICY_ARCHIVED"})
			continue
		}
		target, resolveErr := s.resolveTarget(t.ctx, p.AccountID, p.SourceGroupID, p.Condition)
		if resolveErr != nil || !target.CanProbe {
			reason := "QUALITY_TARGET_OUT_OF_SCOPE"
			var e *Error
			if errors.As(resolveErr, &e) {
				reason = e.Code
			}
			items = append(items, Submission{PolicyID: id, Submission: "skipped", Reason: reason})
			continue
		}
		candidate := p
		candidate.SetTarget(target)
		if candidate.ScopeHash != p.ScopeHash {
			items = append(items, Submission{PolicyID: id, Submission: "skipped", Reason: "QUALITY_SOURCE_CONTEXT_INVALID"})
			continue
		}
		sub, err := t.Enqueue(p, batch, "manual", actor, nil)
		if err != nil {
			return nil, err
		}
		items = append(items, sub)
	}
	return map[string]any{"batch_id": batch, "items": items}, nil
}
func (s *Service) Cancel(t *Tx, id int64, reason string, actor int64) (any, error) {
	r, err := t.Run(id, true)
	if err != nil {
		return nil, err
	}
	effective := r.Status == "queued" || r.Status == "running"
	if effective {
		_, err = t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET cancel_requested=TRUE,status=CASE WHEN status='queued' THEN 'cancelled' ELSE status END,finished_at=CASE WHEN status='queued' THEN NOW() ELSE finished_at END,reason_code='cancel_requested' WHERE id=$1`, id)
		if err != nil {
			return nil, err
		}
		p, e := t.Policy(r.PolicyID, false)
		if e != nil {
			return nil, e
		}
		if err = t.AddEvent(&p, &id, "cancel_requested", p.State.Name, reason, actor); err != nil {
			return nil, err
		}
	}
	updated, err := t.Run(id, false)
	if err != nil {
		return nil, err
	}
	return map[string]any{"run": updated, "cancel_requested": effective, "cancellation_effective": updated.Status == "cancelled", "upstream_dispatched": updated.UpstreamDispatchedAt != nil}, nil
}

type PolicyFilter struct {
	AccountID, SourceGroupID                       int64
	ModelID, State, Freshness, Eligibility, Search string
	ControlEnabled, AutoEnabled                    *bool
	Archived                                       bool
}

func filterPolicies(list []Policy, f PolicyFilter, settings Settings, now time.Time) []Policy {
	out := []Policy{}
	for _, p := range list {
		p = Present(p, settings, now)
		if !f.Archived && p.ArchivedAt != nil {
			continue
		}
		if f.AccountID > 0 && f.AccountID != p.AccountID {
			continue
		}
		if f.SourceGroupID > 0 {
			found := p.SourceGroupID == f.SourceGroupID
			for _, id := range p.CompatibleSourceGroupIDs {
				found = found || id == f.SourceGroupID
			}
			if !found {
				continue
			}
		}
		if f.ModelID != "" && p.Condition.ModelID != f.ModelID {
			continue
		}
		if f.State != "" && p.State.Name != f.State {
			continue
		}
		if f.Freshness != "" && p.State.Freshness != f.Freshness {
			continue
		}
		if f.Eligibility != "" && p.Eligibility != f.Eligibility {
			continue
		}
		if f.ControlEnabled != nil && p.Rules.ControlEnabled != *f.ControlEnabled {
			continue
		}
		if f.AutoEnabled != nil && p.Rules.AutoCheckEnabled != *f.AutoEnabled {
			continue
		}
		if f.Search != "" && !strings.Contains(strings.ToLower(p.AccountName), strings.ToLower(f.Search)) {
			continue
		}
		out = append(out, p)
	}
	return out
}
func page[T any](items []T, number, size int) map[string]any {
	start := min(len(items), (number-1)*size)
	end := min(len(items), start+size)
	return map[string]any{"items": items[start:end], "total": len(items), "page": number, "page_size": size, "pages": max(1, (len(items)+size-1)/size)}
}
func (s *Service) ListTargets(ctx context.Context) ([]Target, error) { return s.provider.Targets(ctx) }

func (s *Service) reconcile(ctx context.Context) {
	v, err := s.Read(ctx, func(t *Tx) (any, error) { return t.Policies(false) })
	if err != nil {
		return
	}
	policies := v.([]Policy)
	for _, old := range policies {
		if ctx.Err() != nil {
			return
		}
		target, e := s.resolveTarget(ctx, old.AccountID, old.SourceGroupID, old.Condition)
		eligibility := "eligible"
		if e != nil {
			var qe *Error
			if !errors.As(e, &qe) || qe.Status >= 500 {
				continue
			}
			eligibility = "source_context_invalid"
			if qe.Code == "QUALITY_TARGET_OUT_OF_SCOPE" {
				eligibility = "out_of_scope"
			}
		}
		candidate := old
		if e == nil {
			candidate.SetTarget(target)
		}
		if old.Eligibility == eligibility && candidate.ScopeHash == old.ScopeHash {
			continue
		}
		_, err = s.Write(ctx, func(t *Tx) (any, error) {
			p, err := t.Policy(old.ID, true)
			if err != nil {
				return nil, err
			}
			if p.Revision != old.Revision {
				return nil, nil
			}
			oldState := p.State.Name
			p.Revision++
			p.ControlEpoch++
			p.Eligibility = eligibility
			if e == nil {
				p.SetTarget(target)
			}
			p.State = NewState(t.Settings.ControlEpoch, p.ControlEpoch)
			schedule(&p, t.Settings, t.Now, true)
			if err = t.CancelUnsent(p.ID, false, "scope_changed"); err != nil {
				return nil, err
			}
			if err = t.SavePolicy(&p); err != nil {
				return nil, err
			}
			return nil, t.AddEvent(&p, nil, "scope_changed", oldState, eligibility, 0)
		})
		if err != nil {
			slog.Warn("account_quality_reconcile_failed", "policy_id", old.ID, "error_type", errorCode(err))
		}
	}
}
func errorCode(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "QUALITY_PERSISTENCE_UNAVAILABLE"
}
