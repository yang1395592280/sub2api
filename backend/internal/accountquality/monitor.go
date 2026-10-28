package accountquality

import (
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Adapted from manxue-ai app/web/app.js renderTimeline and Monitor.state,
// revision 5da79674417dd0748682f09b7b7467efd1968ea8, Apache-2.0.
// Changes: scoped server aggregation, valid-answer denominators and bounded
// samples. Attribution: third_party/manxue-ai/NOTICE and upstream/LICENSE.
type Summary struct {
	Total          int      `json:"total"`
	Completed      int      `json:"completed"`
	Passed         int      `json:"passed"`
	Incorrect      int      `json:"incorrect"`
	Unknown        int      `json:"unknown"`
	Errors         int      `json:"errors"`
	Queued         int      `json:"queued"`
	Running        int      `json:"running"`
	Cancelled      int      `json:"cancelled"`
	Interrupted    int      `json:"interrupted"`
	Skipped        int      `json:"skipped"`
	ValidAnswers   int      `json:"valid_answers"`
	PassRate       *float64 `json:"pass_rate"`
	AnswerCoverage *float64 `json:"answer_coverage"`
}

func (s *Summary) Add(r Run) {
	s.Total++
	switch r.Status {
	case "completed":
		s.Completed++
		if r.Verdict == nil {
			s.Unknown++
		} else {
			switch *r.Verdict {
			case "passed":
				s.Passed++
			case "incorrect":
				s.Incorrect++
			case "error":
				s.Errors++
			default:
				s.Unknown++
			}
		}
	case "queued":
		s.Queued++
	case "running":
		s.Running++
	case "cancelled":
		s.Cancelled++
	case "interrupted":
		s.Interrupted++
	case "skipped":
		s.Skipped++
	}
}
func (s *Summary) Finish() {
	s.ValidAnswers = s.Passed + s.Incorrect
	if s.ValidAnswers > 0 {
		rate := 100 * float64(s.Passed) / float64(s.ValidAnswers)
		s.PassRate = &rate
	}
	if s.Total > 0 {
		rate := 100 * float64(s.ValidAnswers) / float64(s.Total)
		s.AnswerCoverage = &rate
	}
}

type Bucket struct {
	Index        int            `json:"index"`
	StartAt      time.Time      `json:"start_at"`
	EndAt        time.Time      `json:"end_at"`
	Status       string         `json:"status"`
	Total        int            `json:"total"`
	StatusCounts map[string]int `json:"status_counts"`
	SampleRunIDs []int64        `json:"sample_run_ids"`
	HasMore      bool           `json:"has_more"`
}
type Monitor struct {
	PolicyID      int64     `json:"policy_id"`
	Revision      int64     `json:"revision"`
	ScopeHash     string    `json:"scope_hash"`
	GlobalEpoch   int64     `json:"global_control_epoch"`
	PolicyEpoch   int64     `json:"policy_control_epoch"`
	EvidenceView  string    `json:"evidence_view"`
	Eligibility   string    `json:"eligibility"`
	AsOf          time.Time `json:"as_of"`
	WindowStart   time.Time `json:"window_start"`
	WindowEnd     time.Time `json:"window_end"`
	BucketSeconds int       `json:"bucket_seconds"`
	Complete      bool      `json:"complete"`
	Summary       Summary   `json:"summary"`
	Buckets       []Bucket  `json:"buckets"`
}

var statusPriority = map[string]int{"empty": 0, "passed": 1, "skipped": 2, "cancelled": 3, "interrupted": 4, "unknown": 5, "incorrect": 6, "error": 7, "queued": 8, "running": 9}

func timelineStatus(r Run) string {
	if r.Status != "completed" {
		return r.Status
	}
	if r.Verdict == nil {
		return "unknown"
	}
	return *r.Verdict
}
func BuildMonitor(p Policy, settings Settings, now time.Time, seconds int, view string, runs []Run) (Monitor, error) {
	if seconds < 300 || seconds > 86400 {
		return Monitor{}, Invalid("bucket_seconds", "时间桶必须为300–86400秒")
	}
	if view != "current_epoch" && view != "current_scope" {
		return Monitor{}, Invalid("evidence_view", "不支持的证据视图")
	}
	p = Present(p, settings, now)
	start := now.Add(-24 * time.Hour)
	m := Monitor{PolicyID: p.ID, Revision: p.Revision, ScopeHash: p.ScopeHash, GlobalEpoch: settings.ControlEpoch, PolicyEpoch: p.ControlEpoch, EvidenceView: view, Eligibility: p.Eligibility, AsOf: now, WindowStart: start, WindowEnd: now, BucketSeconds: seconds, Complete: true, Buckets: []Bucket{}}
	interval := time.Duration(seconds) * time.Second
	for at := start; at.Before(now); at = at.Add(interval) {
		end := at.Add(interval)
		if end.After(now) {
			end = now
		}
		m.Buckets = append(m.Buckets, Bucket{Index: len(m.Buckets), StartAt: at, EndAt: end, Status: "empty", StatusCounts: map[string]int{}, SampleRunIDs: []int64{}})
	}
	seen := map[int64]bool{}
	for _, r := range runs {
		if r.PolicyID != p.ID || r.ScopeHash != p.ScopeHash || seen[r.ID] {
			continue
		}
		if view == "current_epoch" && (r.Snapshot.State.GlobalEpoch != settings.ControlEpoch || r.Snapshot.ControlEpoch != p.ControlEpoch) {
			continue
		}
		at := r.QueuedAt
		if r.StartedAt != nil {
			at = *r.StartedAt
		}
		if at.Before(start) || at.After(now) {
			continue
		}
		seen[r.ID] = true
		m.Summary.Add(r)
		index := min(len(m.Buckets)-1, int(at.Sub(start)/interval))
		b := &m.Buckets[index]
		b.Total++
		status := timelineStatus(r)
		b.StatusCounts[status]++
		if statusPriority[status] > statusPriority[b.Status] {
			b.Status = status
		}
		if len(b.SampleRunIDs) < 5 {
			b.SampleRunIDs = append(b.SampleRunIDs, r.ID)
		}
		b.HasMore = b.Total > len(b.SampleRunIDs)
	}
	m.Summary.Finish()
	return m, nil
}
func (t *Tx) MonitorRuns(policies []Policy) ([]Run, error) {
	if len(policies) == 0 {
		return []Run{}, nil
	}
	ids := make([]int64, 0, len(policies))
	for _, p := range policies {
		ids = append(ids, p.ID)
	}
	rows, err := t.sql.QueryContext(t.ctx, `SELECT id,policy_id,snapshot->>'scope_hash',COALESCE((snapshot->'state'->>'global_control_epoch')::bigint,0),COALESCE((snapshot->>'policy_control_epoch')::bigint,0),status,verdict,queued_at,started_at FROM account_quality_test_runs WHERE policy_id=ANY($1) AND COALESCE(started_at,queued_at)>=NOW()-INTERVAL '24 hours' AND COALESCE(started_at,queued_at)<=NOW() ORDER BY id DESC`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var r Run
		if err = rows.Scan(&r.ID, &r.PolicyID, &r.ScopeHash, &r.Snapshot.State.GlobalEpoch, &r.Snapshot.ControlEpoch, &r.Status, &r.Verdict, &r.QueuedAt, &r.StartedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (t *Tx) Monitor(id int64, seconds int, view string) (Monitor, error) {
	p, err := t.Policy(id, false)
	if err != nil {
		return Monitor{}, err
	}
	if seconds == 0 {
		seconds = p.Rules.IntervalSeconds
	}
	runs, err := t.MonitorRuns([]Policy{p})
	if err != nil {
		return Monitor{}, err
	}
	return BuildMonitor(p, t.Settings, t.Now, seconds, view, runs)
}
func (t *Tx) RuntimeHealth() (map[string]any, error) {
	var queued, running int
	if err := t.sql.QueryRowContext(t.ctx, `SELECT count(id) FILTER(WHERE status='queued'),count(id) FILTER(WHERE status='running') FROM account_quality_test_runs WHERE status IN ('queued','running')`).Scan(&queued, &running); err != nil {
		return nil, err
	}
	return map[string]any{"queue_depth": queued, "running_count": running, "store_status": "healthy", "last_sync_at": t.Now}, nil
}
func (s *Service) Overview(t *Tx, f PolicyFilter, targets []Target) (any, error) {
	all, err := t.Policies(false)
	if err != nil {
		return nil, err
	}
	if f.Eligibility == "" {
		f.Eligibility = "eligible"
	}
	policies := filterPolicies(all, f, t.Settings, t.Now)
	runs, err := t.MonitorRuns(policies)
	if err != nil {
		return nil, err
	}
	current := map[int64]Policy{}
	states := map[string]int{}
	freshness := map[string]int{}
	effective, outOfScope, invalid := 0, 0, 0
	for _, p := range policies {
		current[p.ID] = p
		states[p.State.Name]++
		freshness[p.State.Freshness]++
		if p.Control.Effective {
			effective++
		}
		if p.Eligibility == "out_of_scope" {
			outOfScope++
		}
		if p.Eligibility == "source_context_invalid" {
			invalid++
		}
	}
	var summary Summary
	for _, r := range runs {
		p, ok := current[r.PolicyID]
		if ok && r.ScopeHash == p.ScopeHash && r.Snapshot.State.GlobalEpoch == t.Settings.ControlEpoch && r.Snapshot.ControlEpoch == p.ControlEpoch {
			summary.Add(r)
		}
	}
	summary.Finish()
	eligible := map[int64]bool{}
	configured := map[int64]bool{}
	for _, target := range targets {
		if f.AccountID > 0 && f.AccountID != target.AccountID {
			continue
		}
		if f.SourceGroupID > 0 {
			found := false
			for _, id := range target.CompatibleSourceGroupIDs {
				found = found || id == f.SourceGroupID
			}
			if !found {
				continue
			}
		}
		if f.Search != "" && !containsFold(target.AccountName, f.Search) {
			continue
		}
		eligible[target.AccountID] = true
	}
	for _, p := range all {
		if eligible[p.AccountID] && p.Eligibility == "eligible" {
			configured[p.AccountID] = true
		}
	}
	budget, err := t.Budget("global", 0)
	if err != nil {
		return nil, err
	}
	health, err := t.RuntimeHealth()
	if err != nil {
		return nil, err
	}
	return map[string]any{"snapshot": map[string]any{"as_of": t.Now, "window_start": t.Now.Add(-24 * time.Hour), "window_end": t.Now, "complete": true, "applies_to": RoutingMode}, "inventory": map[string]any{"eligible_account_count": len(eligible), "configured_account_count": len(configured), "unconfigured_account_count": len(eligible) - len(configured), "inventory_filters": []string{"source_group_id", "account_id", "search"}}, "policies": map[string]any{"total": len(policies), "state_counts": states, "freshness_counts": freshness, "effective_control_count": effective, "out_of_scope_count": outOfScope, "source_context_invalid_count": invalid}, "runs_24h": map[string]any{"summary": summary, "evidence_view": "current_epoch"}, "budget": budget, "health": health}, nil
}
func containsFold(a, b string) bool { return strings.Contains(strings.ToLower(a), strings.ToLower(b)) }

type Warning struct {
	Code      string  `json:"code"`
	Severity  string  `json:"severity"`
	PolicyIDs []int64 `json:"policy_ids"`
	Message   string  `json:"message"`
}

func Capacity(policies []Policy, settings Settings) map[string]any {
	configured, enabled := 0.0, 0.0
	per := []map[string]any{}
	warnings := []Warning{}
	for _, p := range policies {
		if p.ArchivedAt != nil || p.Eligibility != "eligible" || !p.Rules.AutoCheckEnabled {
			continue
		}
		r := p.Rules
		demand := 86400.0 / float64(r.IntervalSeconds)
		if p.Condition.BenchmarkID == "dual" {
			demand *= 3
		}
		configured += demand
		if settings.AutomationEnabled {
			enabled += demand
		}
		span := max((r.FailureThreshold-1)*r.IntervalSeconds, (r.RecoveryThreshold-1)*r.RecoveryIntervalSeconds)
		slack := r.EvidenceWindowSeconds - span
		per = append(per, map[string]any{"policy_id": p.ID, "daily_call_limit": r.DailyCallLimit, "ordinary_calls_per_day": demand, "ideal_failure_span_seconds": (r.FailureThreshold - 1) * r.IntervalSeconds, "ideal_recovery_span_seconds": (r.RecoveryThreshold - 1) * r.RecoveryIntervalSeconds, "window_slack_seconds": slack})
		if demand > float64(r.DailyCallLimit) {
			warnings = append(warnings, Warning{"BUDGET_DEFICIT", "warning", []int64{p.ID}, "单策略日预算不足以维持配置周期"})
		}
		if slack < r.TimeoutSeconds+settings.QueueTimeoutSeconds+r.JitterSeconds {
			warnings = append(warnings, Warning{"WINDOW_DELAY_RISK", "warning", []int64{p.ID}, "队列、超时或抖动可能使阈值样本超出窗口"})
		}
	}
	var coverage *float64
	if configured > 0 {
		v := min(100.0, 100*float64(settings.GlobalDailyCallLimit)/configured)
		coverage = &v
		if configured > float64(settings.GlobalDailyCallLimit) {
			warnings = append(warnings, Warning{"BUDGET_DEFICIT", "warning", []int64{}, fmt.Sprintf("基础需求约%.0f次/日，全局上限%d；公平分配可能无法累积窗口内证据", configured, settings.GlobalDailyCallLimit)})
		}
		warnings = append(warnings, Warning{"RECOVERY_BUDGET_SHARED", "warning", []int64{}, "手动、定时和恢复共用预算，理论余量不是恢复专用预留"})
	}
	return map[string]any{"configured_demand": configured, "enabled_demand": enabled, "ordinary_calls_per_day": configured, "global_limit": settings.GlobalDailyCallLimit, "planned_call_capacity_percent": coverage, "per_policy": per, "warnings": warnings}
}
