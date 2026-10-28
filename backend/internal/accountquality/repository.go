package accountquality

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Store struct {
	db       *sql.DB
	timezone string
}
type Tx struct {
	sql      *sql.Tx
	ctx      context.Context
	Settings Settings
	Now      time.Time
	timezone string
}

func NewStore(db *sql.DB, timezone string) *Store {
	if _, err := time.LoadLocation(timezone); err != nil {
		timezone = "Asia/Shanghai"
	}
	return &Store{db: db, timezone: timezone}
}
func (s *Store) Transaction(ctx context.Context, readOnly bool, f func(*Tx) (any, error)) (any, error) {
	opts := &sql.TxOptions{ReadOnly: readOnly}
	if readOnly {
		opts.Isolation = sql.LevelRepeatableRead
	}
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	q := `SELECT value, NOW() FROM settings WHERE key=$1`
	if !readOnly {
		q += ` FOR UPDATE`
	}
	t := &Tx{sql: tx, ctx: ctx, timezone: s.timezone}
	var raw string
	if err = tx.QueryRowContext(ctx, q, SettingsKey).Scan(&raw, &t.Now); err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(raw), &t.Settings); err != nil {
		return nil, err
	}
	if err = t.Settings.Validate(); err != nil {
		return nil, err
	}
	v, err := f(t)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return v, nil
}
func (t *Tx) SaveSettings(s Settings) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = t.sql.ExecContext(t.ctx, `UPDATE settings SET value=$2,updated_at=NOW() WHERE key=$1`, SettingsKey, string(b))
	if err == nil {
		t.Settings = s
	}
	return err
}

const policyColumns = `id,document,revision,policy_control_epoch,eligibility_status,control_scope_hash,scope_hash,next_run_at,archived_at,created_at,updated_at`

func scanPolicy(row interface{ Scan(...any) error }) (Policy, error) {
	var p Policy
	var b []byte
	var id, rev, epoch int64
	var eligibility, controlHash, scope string
	var next, archived *time.Time
	var created, updated time.Time
	err := row.Scan(&id, &b, &rev, &epoch, &eligibility, &controlHash, &scope, &next, &archived, &created, &updated)
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(b, &p); err != nil {
		return p, err
	}
	p.ID = id
	p.Revision = rev
	p.ControlEpoch = epoch
	p.Eligibility = eligibility
	p.ControlScopeHash = controlHash
	p.ScopeHash = scope
	p.NextRunAt = next
	p.ArchivedAt = archived
	p.CreatedAt = created
	p.UpdatedAt = updated
	return p, nil
}
func (t *Tx) Policy(id int64, lock bool) (Policy, error) {
	q := `SELECT ` + policyColumns + ` FROM account_quality_policies WHERE id=$1`
	if lock {
		q += ` FOR UPDATE`
	}
	p, err := scanPolicy(t.sql.QueryRowContext(t.ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		err = Fail(404, "QUALITY_POLICY_NOT_FOUND", "策略不存在")
	}
	return p, err
}
func (t *Tx) Policies(archived bool) ([]Policy, error) {
	q := `SELECT ` + policyColumns + ` FROM account_quality_policies`
	if !archived {
		q += ` WHERE archived_at IS NULL`
	}
	q += ` ORDER BY id DESC`
	rows, err := t.sql.QueryContext(t.ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Policy{}
	for rows.Next() {
		p, e := scanPolicy(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (t *Tx) SavePolicy(p *Policy) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if p.ID == 0 {
		err = t.sql.QueryRowContext(t.ctx, `INSERT INTO account_quality_policies(account_id,source_group_id,control_scope_hash,scope_hash,revision,policy_control_epoch,eligibility_status,document,next_run_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,created_at,updated_at`, p.AccountID, p.SourceGroupID, p.ControlScopeHash, p.ScopeHash, p.Revision, p.ControlEpoch, p.Eligibility, string(b), p.NextRunAt).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	} else {
		_, err = t.sql.ExecContext(t.ctx, `UPDATE account_quality_policies SET control_scope_hash=$2,scope_hash=$3,revision=$4,policy_control_epoch=$5,eligibility_status=$6,document=$7,next_run_at=$8,archived_at=$9,updated_at=NOW() WHERE id=$1`, p.ID, p.ControlScopeHash, p.ScopeHash, p.Revision, p.ControlEpoch, p.Eligibility, string(b), p.NextRunAt, p.ArchivedAt)
		p.UpdatedAt = t.Now
	}
	var pg *pq.Error
	if errors.As(err, &pg) && pg.Code == "23505" {
		return Fail(409, "QUALITY_SCOPE_CONFLICT", "此账号和有效条件已有活动策略")
	}
	return err
}
func (t *Tx) AddEvent(p *Policy, runID *int64, eventType, oldState, reason string, actor int64) error {
	e := Event{RunID: runID, Type: eventType, OldState: oldState, Reason: reason, ActorID: actor, GlobalEpoch: t.Settings.ControlEpoch, CreatedAt: t.Now}
	if p != nil {
		e.PolicyID = &p.ID
		e.AccountID = &p.AccountID
		e.NewState = p.State.Name
		e.Revision = p.Revision
		e.PolicyEpoch = p.ControlEpoch
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = t.sql.ExecContext(t.ctx, `INSERT INTO account_quality_events(policy_id,account_id,run_id,event_type,document) VALUES($1,$2,$3,$4,$5)`, e.PolicyID, e.AccountID, runID, eventType, string(b))
	return err
}
func (t *Tx) CancelUnsent(policyID int64, automaticOnly bool, reason string) error {
	q := `UPDATE account_quality_test_runs SET cancel_requested=TRUE,reason_code=$1,status=CASE WHEN status='queued' THEN 'cancelled' ELSE status END,finished_at=CASE WHEN status='queued' THEN NOW() ELSE finished_at END WHERE status IN ('queued','running') AND upstream_dispatched_at IS NULL`
	args := []any{reason}
	if policyID > 0 {
		q += ` AND policy_id=$2`
		args = append(args, policyID)
	}
	if automaticOnly {
		q += ` AND trigger_source IN ('scheduled','recovery')`
	}
	_, err := t.sql.ExecContext(t.ctx, q, args...)
	return err
}

const runColumns = `id,batch_id,policy_id,account_id,snapshot,trigger_source,actor_user_id,status,verdict,reason_code,scheduled_slot,lease_token,lease_expires_at,cancel_requested,upstream_dispatched_at,queued_at,started_at,finished_at,queue_expires_at,result,response_text,response_expires_at,state_applied,state_skip_reason`

func scanRun(row interface{ Scan(...any) error }) (Run, error) {
	var r Run
	var snapshot, result []byte
	var lease sql.NullString
	err := row.Scan(&r.ID, &r.BatchID, &r.PolicyID, &r.AccountID, &snapshot, &r.TriggerSource, &r.ActorID, &r.Status, &r.Verdict, &r.ReasonCode, &r.ScheduledSlot, &lease, &r.LeaseExpiresAt, &r.CancelRequested, &r.UpstreamDispatchedAt, &r.QueuedAt, &r.StartedAt, &r.FinishedAt, &r.QueueExpiresAt, &result, &r.ResponseText, &r.ResponseExpiresAt, &r.StateApplied, &r.StateSkipReason)
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal(snapshot, &r.Snapshot); err != nil {
		return r, err
	}
	if err = json.Unmarshal(result, &r.Result); err != nil {
		return r, err
	}
	r.ScopeHash = r.Snapshot.ScopeHash
	r.LeaseToken = lease.String
	return r, nil
}
func (t *Tx) Run(id int64, lock bool) (Run, error) {
	q := `SELECT ` + runColumns + ` FROM account_quality_test_runs WHERE id=$1`
	if lock {
		q += ` FOR UPDATE`
	}
	r, err := scanRun(t.sql.QueryRowContext(t.ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		err = Fail(404, "QUALITY_RUN_NOT_FOUND", "检测记录不存在")
	}
	return r, err
}

type Submission struct {
	PolicyID   int64  `json:"policy_id"`
	Submission string `json:"submission"`
	RunID      *int64 `json:"run_id"`
	Reason     string `json:"reason,omitempty"`
}

func (t *Tx) Enqueue(p Policy, batch, source string, actor int64, slot *time.Time) (Submission, error) {
	sub := Submission{PolicyID: p.ID, Submission: "skipped"}
	var existing int64
	err := t.sql.QueryRowContext(t.ctx, `SELECT id FROM account_quality_test_runs WHERE policy_id=$1 AND status IN ('queued','running')`, p.ID).Scan(&existing)
	if err == nil {
		sub.Submission = "existing"
		sub.RunID = &existing
		return sub, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return sub, err
	}
	var depth int
	if err = t.sql.QueryRowContext(t.ctx, `SELECT count(id) FROM account_quality_test_runs WHERE status='queued'`).Scan(&depth); err != nil {
		return sub, err
	}
	if depth >= t.Settings.QueueLimit {
		sub.Reason = "QUALITY_QUEUE_FULL"
		return sub, nil
	}
	global, err := t.Budget("global", 0)
	if err != nil {
		return sub, err
	}
	local, err := t.Budget("policy", p.ID)
	if err != nil {
		return sub, err
	}
	if global.Consumed+global.Reserved >= t.Settings.GlobalDailyCallLimit || local.Consumed+local.Reserved >= p.Rules.DailyCallLimit {
		sub.Reason = "QUALITY_DAILY_BUDGET_EXCEEDED"
		return sub, nil
	}
	p = Present(p, t.Settings, t.Now)
	b, err := json.Marshal(p)
	if err != nil {
		return sub, err
	}
	var id int64
	err = t.sql.QueryRowContext(t.ctx, `INSERT INTO account_quality_test_runs(batch_id,policy_id,account_id,snapshot,trigger_source,actor_user_id,scheduled_slot,queue_expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING RETURNING id`, batch, p.ID, p.AccountID, string(b), source, actor, slot, t.Now.Add(time.Duration(t.Settings.QueueTimeoutSeconds)*time.Second)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		sub.Reason = "QUALITY_TASK_CONFLICT"
		return sub, nil
	}
	if err != nil {
		return sub, err
	}
	sub.Submission = "queued"
	sub.RunID = &id
	return sub, t.AddEvent(&p, &id, "submitted", p.State.Name, source, actor)
}

type Budget struct {
	Date      string `json:"budget_date"`
	Timezone  string `json:"timezone"`
	Limit     int    `json:"limit"`
	Consumed  int    `json:"consumed"`
	Reserved  int    `json:"reserved"`
	Remaining int    `json:"remaining"`
}

func (t *Tx) Budget(scope string, id int64) (Budget, error) {
	b := Budget{Timezone: t.timezone}
	err := t.sql.QueryRowContext(t.ctx, `SELECT to_char(NOW() AT TIME ZONE $1,'YYYY-MM-DD')`, t.timezone).Scan(&b.Date)
	if err != nil {
		return b, err
	}
	err = t.sql.QueryRowContext(t.ctx, `SELECT consumed_calls,reserved_calls FROM account_quality_daily_budgets WHERE budget_date=$1::date AND scope_type=$2 AND scope_id=$3`, b.Date, scope, id).Scan(&b.Consumed, &b.Reserved)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if scope == "global" {
		b.Limit = t.Settings.GlobalDailyCallLimit
	}
	b.Remaining = max(0, b.Limit-b.Consumed-b.Reserved)
	return b, err
}

type RunFilter struct {
	PolicyID, AccountID, RunID                   int64
	BatchID, ScopeHash, Status, Verdict, Trigger string
	From, To                                     *time.Time
	IncludeEnd                                   bool
	GlobalEpoch, PolicyEpoch                     *int64
	Limit, Offset                                int
}

func (t *Tx) Runs(f RunFilter) ([]Run, int, error) {
	where := []string{"TRUE"}
	args := []any{}
	add := func(expr string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	if f.PolicyID > 0 {
		add("policy_id=$%d", f.PolicyID)
	}
	if f.AccountID > 0 {
		add("account_id=$%d", f.AccountID)
	}
	if f.RunID > 0 {
		add("id=$%d", f.RunID)
	}
	if f.BatchID != "" {
		add("batch_id=$%d", f.BatchID)
	}
	if f.ScopeHash != "" {
		add("snapshot->>'scope_hash'=$%d", f.ScopeHash)
	}
	if f.Status != "" {
		add("status=$%d", f.Status)
	}
	if f.Verdict != "" {
		add("verdict=$%d", f.Verdict)
	}
	if f.Trigger != "" {
		add("trigger_source=$%d", f.Trigger)
	}
	if f.From != nil {
		add("COALESCE(started_at,queued_at)>=$%d", *f.From)
	}
	if f.To != nil {
		op := "<"
		if f.IncludeEnd {
			op = "<="
		}
		add("COALESCE(started_at,queued_at)"+op+"$%d", *f.To)
	}
	if f.GlobalEpoch != nil {
		add("(snapshot->'state'->>'global_control_epoch')::bigint=$%d", *f.GlobalEpoch)
	}
	if f.PolicyEpoch != nil {
		add("(snapshot->'state'->>'policy_control_epoch')::bigint=$%d", *f.PolicyEpoch)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := t.sql.QueryRowContext(t.ctx, `SELECT count(id) FROM account_quality_test_runs WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	q := `SELECT ` + runColumns + ` FROM account_quality_test_runs WHERE ` + clause + fmt.Sprintf(` ORDER BY id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := t.sql.QueryContext(t.ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		r, e := scanRun(rows)
		if e != nil {
			return nil, 0, e
		}
		r.ResponseText = nil
		out = append(out, r)
	}
	return out, total, rows.Err()
}
func (t *Tx) Samples(p Policy) ([]Sample, error) {
	rows, err := t.sql.QueryContext(t.ctx, `SELECT finished_at,verdict FROM account_quality_test_runs WHERE policy_id=$1 AND state_applied AND snapshot->>'scope_hash'=$2 AND (snapshot->'state'->>'global_control_epoch')::bigint=$3 AND (snapshot->'state'->>'policy_control_epoch')::bigint=$4 AND finished_at>=NOW()-make_interval(secs=>$5) AND verdict IN ('passed','incorrect') ORDER BY finished_at,id`, p.ID, p.ScopeHash, t.Settings.ControlEpoch, p.ControlEpoch, p.Rules.EvidenceWindowSeconds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Sample{}
	for rows.Next() {
		var v Sample
		if err = rows.Scan(&v.At, &v.Verdict); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (t *Tx) Events(policy, account, run int64, eventType string, from, to *time.Time, limit, offset int) ([]Event, int, error) {
	where := ` WHERE ($1::bigint=0 OR policy_id=$1) AND ($2::bigint=0 OR account_id=$2) AND ($3::bigint=0 OR run_id=$3) AND ($4='' OR event_type=$4) AND ($5::timestamptz IS NULL OR created_at>=$5) AND ($6::timestamptz IS NULL OR created_at<$6)`
	args := []any{policy, account, run, eventType, from, to}
	var total int
	if err := t.sql.QueryRowContext(t.ctx, `SELECT count(id) FROM account_quality_events`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := t.sql.QueryContext(t.ctx, `SELECT id,document,created_at,run_id FROM account_quality_events`+where+` ORDER BY id DESC LIMIT $7 OFFSET $8`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var b []byte
		var id int64
		var at time.Time
		var runID *int64
		if err = rows.Scan(&id, &b, &at, &runID); err != nil {
			return nil, 0, err
		}
		if err = json.Unmarshal(b, &e); err != nil {
			return nil, 0, err
		}
		e.ID = id
		e.CreatedAt = at
		e.RunID = runID
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// Idempotent executes the mutation and replay response in the same short SQL
// transaction. It cannot fall back to creating new paid tasks on store errors.
func (t *Tx) Idempotent(actor int64, action, key string, payload any, status int, f func() (any, error)) (any, int, bool, error) {
	if strings.TrimSpace(key) == "" {
		return nil, 0, false, Fail(400, "IDEMPOTENCY_KEY_REQUIRED", "请提供Idempotency-Key")
	}
	if len(key) > 128 || strings.ContainsAny(key, "\r\n\x00") {
		return nil, 0, false, Fail(400, "IDEMPOTENCY_KEY_INVALID", "幂等键格式错误")
	}
	scope := fmt.Sprintf("account-quality:%d:%s", actor, action)
	hash := Hash(key)
	fingerprint := Hash(payload)
	var id int64
	var oldFingerprint, oldStatus string
	var oldBody sql.NullString
	var oldHTTP sql.NullInt64
	var expires time.Time
	err := t.sql.QueryRowContext(t.ctx, `SELECT id,request_fingerprint,status,response_body,response_status,expires_at FROM idempotency_records WHERE scope=$1 AND idempotency_key_hash=$2 FOR UPDATE`, scope, hash).Scan(&id, &oldFingerprint, &oldStatus, &oldBody, &oldHTTP, &expires)
	if err == nil && expires.After(t.Now) {
		if fingerprint != oldFingerprint {
			return nil, 0, false, Fail(409, "IDEMPOTENCY_KEY_CONFLICT", "同一幂等键对应不同操作")
		}
		if oldStatus != "succeeded" {
			return nil, 0, false, Fail(409, "IDEMPOTENCY_IN_PROGRESS", "操作仍在进行")
		}
		var out any
		if err = json.Unmarshal([]byte(oldBody.String), &out); err != nil {
			return nil, 0, false, err
		}
		return out, int(oldHTTP.Int64), true, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, false, err
	}
	if id > 0 {
		if _, err = t.sql.ExecContext(t.ctx, `DELETE FROM idempotency_records WHERE id=$1`, id); err != nil {
			return nil, 0, false, err
		}
	}
	data, err := f()
	if err != nil {
		return nil, 0, false, err
	}
	body, err := json.Marshal(data)
	if err != nil {
		return nil, 0, false, err
	}
	_, err = t.sql.ExecContext(t.ctx, `INSERT INTO idempotency_records(scope,idempotency_key_hash,request_fingerprint,status,response_status,response_body,expires_at) VALUES($1,$2,$3,'succeeded',$4,$5,$6)`, scope, hash, fingerprint, status, string(body), t.Now.Add(24*time.Hour))
	return data, status, false, err
}

func newBatchID() string { return uuid.NewString() }
