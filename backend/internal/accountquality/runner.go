package accountquality

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) loop(ctx context.Context) {
	defer close(s.done)
	s.Refresh(ctx)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	counter := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			counter++
			if counter%5 == 0 {
				s.Refresh(ctx)
			}
			if counter%30 == 0 {
				s.reconcile(ctx)
			}
			if err := s.tick(ctx, counter%60 == 0); err != nil && ctx.Err() == nil {
				slog.Warn("account_quality_tick_failed", "reason", errorCode(err))
			}
			for i := 0; i < 10; i++ {
				r, err := s.claim(ctx)
				if err != nil {
					if ctx.Err() == nil {
						slog.Warn("account_quality_claim_failed", "reason", errorCode(err))
					}
					break
				}
				if r == nil {
					break
				}
				s.workers.Add(1)
				go func(run Run) { defer s.workers.Done(); s.execute(ctx, run) }(*r)
			}
		}
	}
}

func (s *Service) tick(ctx context.Context, cleanup bool) error {
	_, err := s.store.Transaction(ctx, false, func(t *Tx) (any, error) {
		rows, e := t.sql.QueryContext(t.ctx, `SELECT `+runColumns+` FROM account_quality_test_runs WHERE (status='running' AND lease_expires_at<NOW()) OR (status='queued' AND queue_expires_at<NOW()) ORDER BY id FOR UPDATE`)
		if e != nil {
			return nil, e
		}
		expired := []Run{}
		for rows.Next() {
			r, scanErr := scanRun(rows)
			if scanErr != nil {
				rows.Close()
				return nil, scanErr
			}
			expired = append(expired, r)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		for _, r := range expired {
			if e = t.releaseBudget(r.ID); e != nil {
				return nil, e
			}
			status, reason := "interrupted", "lease_expired"
			if r.Status == "queued" {
				status = "skipped"
				reason = "queue_timeout"
			}
			if _, e = t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET status=$2,reason_code=$3,finished_at=NOW() WHERE id=$1`, r.ID, status, reason); e != nil {
				return nil, e
			}
		}
		if t.Settings.AutomationEnabled {
			policies, e := t.Policies(false)
			if e != nil {
				return nil, e
			}
			for _, p := range policies {
				if !p.Rules.AutoCheckEnabled || p.Eligibility != "eligible" {
					continue
				}
				p = Present(p, t.Settings, t.Now)
				if p.NextRunAt == nil {
					schedule(&p, t.Settings, t.Now, true)
					if e = t.SavePolicy(&p); e != nil {
						return nil, e
					}
				}
				if p.NextRunAt == nil || p.NextRunAt.After(t.Now) {
					continue
				}
				if p.State.Isolated() && p.State.CooldownUntil != nil && t.Now.Before(*p.State.CooldownUntil) {
					p.NextRunAt = p.State.CooldownUntil
					if e = t.SavePolicy(&p); e != nil {
						return nil, e
					}
					continue
				}
				source := "scheduled"
				if p.State.Isolated() {
					source = "recovery"
				}
				slot := *p.NextRunAt
				sub, e := t.Enqueue(p, newBatchID(), source, 0, &slot)
				if e != nil {
					return nil, e
				}
				schedule(&p, t.Settings, t.Now, false)
				if e = t.SavePolicy(&p); e != nil {
					return nil, e
				}
				if sub.Submission == "skipped" {
					if e = t.AddEvent(&p, nil, "submission_skipped", p.State.Name, sub.Reason, 0); e != nil {
						return nil, e
					}
				}
			}
		}
		if cleanup {
			if _, e = t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET response_text=NULL WHERE id IN (SELECT id FROM account_quality_test_runs WHERE response_text IS NOT NULL AND response_expires_at<NOW() AND status NOT IN ('queued','running') ORDER BY id LIMIT 500)`); e != nil {
				return nil, e
			}
			if _, e = t.sql.ExecContext(t.ctx, `DELETE FROM account_quality_events WHERE id IN (SELECT id FROM account_quality_events WHERE created_at<NOW()-make_interval(days=>$1) ORDER BY id LIMIT 500)`, t.Settings.MetadataRetentionDays); e != nil {
				return nil, e
			}
			if _, e = t.sql.ExecContext(t.ctx, `DELETE FROM account_quality_test_runs WHERE id IN (SELECT id FROM account_quality_test_runs WHERE status NOT IN ('queued','running') AND queued_at<NOW()-make_interval(days=>$1) ORDER BY id LIMIT 500)`, t.Settings.MetadataRetentionDays); e != nil {
				return nil, e
			}
			if _, e = t.sql.ExecContext(t.ctx, `DELETE FROM account_quality_daily_budgets WHERE budget_date<(NOW() AT TIME ZONE $1)::date-$2 AND reserved_calls=0`, t.timezone, t.Settings.MetadataRetentionDays); e != nil {
				return nil, e
			}
		}
		return nil, nil
	})
	return err
}
func (s *Service) claim(ctx context.Context) (*Run, error) {
	v, err := s.store.Transaction(ctx, false, func(t *Tx) (any, error) {
		var running int
		if e := t.sql.QueryRowContext(t.ctx, `SELECT count(id) FROM account_quality_test_runs WHERE status='running'`).Scan(&running); e != nil {
			return nil, e
		}
		if running >= t.Settings.GlobalConcurrency {
			return nil, nil
		}
		r, e := scanRun(t.sql.QueryRowContext(t.ctx, `SELECT `+runColumns+` FROM account_quality_test_runs r WHERE status='queued' AND NOT cancel_requested AND queue_expires_at>NOW() AND NOT EXISTS (SELECT 1 FROM account_quality_test_runs active WHERE active.account_id=r.account_id AND active.status='running') ORDER BY queued_at,id FOR UPDATE SKIP LOCKED LIMIT 1`))
		if errors.Is(e, sql.ErrNoRows) {
			return nil, nil
		}
		if e != nil {
			return nil, e
		}
		p, e := t.Policy(r.PolicyID, true)
		if e != nil {
			return nil, e
		}
		reason := validRun(t, p, r)
		if reason != "" {
			_, e = t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET status='cancelled',reason_code=$2,finished_at=NOW() WHERE id=$1`, r.ID, reason)
			return nil, e
		}
		if e = t.reserveBudget(r.ID, p); e != nil {
			var qe *Error
			if errors.As(e, &qe) && qe.Code == "QUALITY_DAILY_BUDGET_EXCEEDED" {
				_, e = t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET status='skipped',reason_code='QUALITY_DAILY_BUDGET_EXCEEDED',finished_at=NOW() WHERE id=$1`, r.ID)
				return nil, e
			}
			return nil, e
		}
		token := uuid.NewString()
		_, e = t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET status='running',started_at=NOW(),lease_token=$2,lease_expires_at=NOW()+INTERVAL '60 seconds' WHERE id=$1`, r.ID, token)
		if e != nil {
			return nil, e
		}
		r, e = t.Run(r.ID, false)
		return &r, e
	})
	if err != nil || v == nil {
		return nil, err
	}
	return v.(*Run), nil
}
func validRun(t *Tx, p Policy, r Run) string {
	if p.ArchivedAt != nil {
		return "policy_archived"
	}
	if p.Eligibility != "eligible" {
		return "QUALITY_TARGET_OUT_OF_SCOPE"
	}
	if p.Revision != r.Snapshot.Revision {
		return "policy_revision_changed"
	}
	if p.ScopeHash != r.ScopeHash {
		return "scope_changed"
	}
	if p.ControlEpoch != r.Snapshot.ControlEpoch || t.Settings.ControlEpoch != r.Snapshot.State.GlobalEpoch {
		return "control_epoch_changed"
	}
	if r.CancelRequested {
		return "cancel_requested"
	}
	if r.TriggerSource != "manual" && (!t.Settings.AutomationEnabled || !p.Rules.AutoCheckEnabled) {
		return "automation_disabled"
	}
	return ""
}
func (t *Tx) reserveBudget(runID int64, p Policy) error {
	g, err := t.Budget("global", 0)
	if err != nil {
		return err
	}
	local, err := t.Budget("policy", p.ID)
	if err != nil {
		return err
	}
	if g.Consumed+g.Reserved >= t.Settings.GlobalDailyCallLimit || local.Consumed+local.Reserved >= p.Rules.DailyCallLimit {
		return Fail(429, "QUALITY_DAILY_BUDGET_EXCEEDED", "今日检测调用额度已用尽")
	}
	for _, scope := range []struct {
		kind string
		id   int64
	}{{"global", 0}, {"policy", p.ID}} {
		_, err = t.sql.ExecContext(t.ctx, `INSERT INTO account_quality_daily_budgets(budget_date,scope_type,scope_id,reserved_calls) VALUES($1::date,$2,$3,1) ON CONFLICT(budget_date,scope_type,scope_id) DO UPDATE SET reserved_calls=account_quality_daily_budgets.reserved_calls+1,updated_at=NOW()`, g.Date, scope.kind, scope.id)
		if err != nil {
			return err
		}
	}
	_, err = t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET budget_date=$2::date,budget_reserved=TRUE,budget_consumed=FALSE WHERE id=$1`, runID, g.Date)
	return err
}
func (t *Tx) releaseBudget(runID int64) error {
	var date sql.NullString
	var reserved, consumed bool
	var policyID int64
	if err := t.sql.QueryRowContext(t.ctx, `SELECT budget_date::text,budget_reserved,budget_consumed,policy_id FROM account_quality_test_runs WHERE id=$1 FOR UPDATE`, runID).Scan(&date, &reserved, &consumed, &policyID); err != nil {
		return err
	}
	if !reserved || consumed {
		return nil
	}
	for _, scope := range []struct {
		kind string
		id   int64
	}{{"global", 0}, {"policy", policyID}} {
		if _, err := t.sql.ExecContext(t.ctx, `UPDATE account_quality_daily_budgets SET reserved_calls=reserved_calls-1,updated_at=NOW() WHERE budget_date=$1::date AND scope_type=$2 AND scope_id=$3 AND reserved_calls>0`, date.String, scope.kind, scope.id); err != nil {
			return err
		}
	}
	_, err := t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET budget_reserved=FALSE WHERE id=$1`, runID)
	return err
}
func (s *Service) dispatch(ctx context.Context, original Run, stage string) error {
	_, err := s.store.Transaction(ctx, false, func(t *Tx) (any, error) {
		r, e := t.Run(original.ID, true)
		if e != nil {
			return nil, e
		}
		p, e := t.Policy(r.PolicyID, true)
		if e != nil {
			return nil, e
		}
		if r.Status != "running" || r.LeaseToken != original.LeaseToken || r.LeaseExpiresAt == nil || !t.Now.Before(*r.LeaseExpiresAt) {
			return nil, Fail(409, "QUALITY_TASK_STALE", "任务租约已失效")
		}
		if reason := validRun(t, p, r); reason != "" {
			return nil, Fail(409, "QUALITY_TASK_STALE", reason)
		}
		expected := []string{"candy"}
		if r.Snapshot.Condition.BenchmarkID == "dual" {
			expected = append(expected, "pelican", "review")
		}
		if len(r.Result.DispatchedStages) >= len(expected) || expected[len(r.Result.DispatchedStages)] != stage {
			return nil, Fail(409, "QUALITY_STAGE_CONFLICT", "检测阶段已发送或顺序错误")
		}
		// Claim reserves the first call; later paid stages obtain their own slot.
		// A committed intent is never replayed after a crash.
		var reserved bool
		if e = t.sql.QueryRowContext(t.ctx, `SELECT budget_reserved FROM account_quality_test_runs WHERE id=$1`, r.ID).Scan(&reserved); e != nil {
			return nil, e
		}
		if !reserved {
			if e = t.reserveBudget(r.ID, p); e != nil {
				return nil, e
			}
		}
		var date string
		if e = t.sql.QueryRowContext(t.ctx, `SELECT budget_date::text FROM account_quality_test_runs WHERE id=$1`, r.ID).Scan(&date); e != nil {
			return nil, e
		}
		g, e := t.Budget("global", 0)
		if e != nil {
			return nil, e
		}
		if date != g.Date {
			if e = t.releaseBudget(r.ID); e != nil {
				return nil, e
			}
			if e = t.reserveBudget(r.ID, p); e != nil {
				return nil, e
			}
		}
		g, e = t.Budget("global", 0)
		if e != nil {
			return nil, e
		}
		local, e := t.Budget("policy", p.ID)
		if e != nil {
			return nil, e
		}
		if g.Consumed+g.Reserved > t.Settings.GlobalDailyCallLimit || local.Consumed+local.Reserved > p.Rules.DailyCallLimit {
			return nil, Fail(429, "QUALITY_DAILY_BUDGET_EXCEEDED", "预算降低后本次名额不可用")
		}
		// The durable intent is the billing boundary. A crash after this commit
		// consumes a call conservatively; the paid request is never replayed.
		for _, scope := range []struct {
			kind string
			id   int64
		}{{"global", 0}, {"policy", p.ID}} {
			updated, e := t.sql.ExecContext(t.ctx, `UPDATE account_quality_daily_budgets SET reserved_calls=reserved_calls-1,consumed_calls=consumed_calls+1,updated_at=NOW() WHERE budget_date=$1::date AND scope_type=$2 AND scope_id=$3 AND reserved_calls>0`, g.Date, scope.kind, scope.id)
			if e != nil {
				return nil, e
			}
			if count, e := updated.RowsAffected(); e != nil || count != 1 {
				return nil, Fail(409, "QUALITY_BUDGET_RESERVATION_LOST", "检测预算预留已失效")
			}
		}
		r.Result.DispatchedStages = append(r.Result.DispatchedStages, stage)
		intent, e := json.Marshal(r.Result)
		if e != nil {
			return nil, e
		}
		_, e = t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET budget_reserved=FALSE,budget_consumed=TRUE,upstream_dispatched_at=COALESCE(upstream_dispatched_at,NOW()),result=$2 WHERE id=$1`, r.ID, string(intent))
		return nil, e
	})
	return err
}
func (s *Service) execute(parent context.Context, run Run) {
	ctx, cancel := context.WithTimeout(parent, time.Duration(run.Snapshot.Rules.TimeoutSeconds)*time.Second)
	defer cancel()
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go func() {
		timer := time.NewTicker(15 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-ctx.Done():
				return
			case <-timer.C:
				beatCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
				var requested bool
				err := s.store.db.QueryRowContext(beatCtx, `UPDATE account_quality_test_runs SET lease_expires_at=NOW()+INTERVAL '60 seconds' WHERE id=$1 AND status='running' AND lease_token=$2 AND lease_expires_at>NOW() RETURNING cancel_requested`, run.ID, run.LeaseToken).Scan(&requested)
				stop()
				if err != nil || requested {
					cancel()
					return
				}
			}
		}
	}()
	result := s.provider.Probe(ctx, run, func(stage string) error { return s.dispatch(ctx, run, stage) })
	finishCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	if err := s.finish(finishCtx, run, result); err != nil {
		slog.Error("account_quality_finish_failed", "run_id", run.ID, "policy_id", run.PolicyID, "reason", errorCode(err))
	}
	s.Refresh(finishCtx)
}
func (s *Service) finish(ctx context.Context, original Run, result Result) error {
	_, err := s.store.Transaction(ctx, false, func(t *Tx) (any, error) {
		r, e := t.Run(original.ID, true)
		if e != nil {
			return nil, e
		}
		if r.Status != "running" || r.LeaseToken != original.LeaseToken || r.LeaseExpiresAt == nil || !t.Now.Before(*r.LeaseExpiresAt) {
			return nil, nil
		}
		p, e := t.Policy(r.PolicyID, true)
		if e != nil {
			return nil, e
		}
		p = Present(p, t.Settings, t.Now)
		oldState := p.State.Name
		skip := validRun(t, p, r)
		status := "completed"
		var verdict *string
		if r.CancelRequested {
			status = "cancelled"
			skip = "cancel_requested"
		} else if r.UpstreamDispatchedAt == nil {
			status = "skipped"
			skip = result.ReasonCode
		} else {
			verdict = &result.Verdict
		}
		applied := skip == "" && status == "completed"
		if applied {
			samples, e := t.Samples(p)
			if e != nil {
				return nil, e
			}
			p.State = Reduce(p.State, p.Rules, t.Now, r.ID, result, samples)
			schedule(&p, t.Settings, t.Now, false)
			if e = t.SavePolicy(&p); e != nil {
				return nil, e
			}
		}
		if e = t.releaseBudget(r.ID); e != nil {
			return nil, e
		}
		var text *string
		var expires *time.Time
		value, e := EncodeEvidence(result)
		if e != nil {
			return nil, e
		}
		if value != "" {
			if result.EvidenceVersion == "" && len(value) > MaxResponseBytes {
				value = strings.ToValidUTF8(value[:MaxResponseBytes], "")
				result.ResponseTruncated = true
			}
			text = &value
			end := t.Now.Add(time.Duration(t.Settings.RawRetentionDays) * 24 * time.Hour)
			expires = &end
		}
		result.DispatchedStages = r.Result.DispatchedStages
		b, e := json.Marshal(result)
		if e != nil {
			return nil, e
		}
		_, e = t.sql.ExecContext(t.ctx, `UPDATE account_quality_test_runs SET status=$3,verdict=$4,reason_code=$5,result=$6,response_text=$7,response_expires_at=$8,state_applied=$9,state_skip_reason=$10,finished_at=NOW() WHERE id=$1 AND lease_token=$2 AND status='running'`, r.ID, r.LeaseToken, status, verdict, result.ReasonCode, string(b), text, expires, applied, skip)
		if e != nil {
			return nil, e
		}
		event := "run_finished"
		if applied && oldState != p.State.Name {
			event = "state_changed"
		}
		return nil, t.AddEvent(&p, &r.ID, event, oldState, result.ReasonCode, r.ActorID)
	})
	return err
}
