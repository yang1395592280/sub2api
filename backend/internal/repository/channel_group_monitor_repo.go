package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type groupMonitorRepository struct{ db *sql.DB }

func NewGroupMonitorRepository(db *sql.DB) service.GroupMonitorRepository {
	return &groupMonitorRepository{db: db}
}
func (r *groupMonitorRepository) List(ctx context.Context) ([]service.GroupMonitor, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT m.id,m.group_id,g.name,g.platform,m.config,m.next_probe_at,m.next_pelican_at,m.lease_until,
 CASE WHEN m.budget_day=(NOW() AT TIME ZONE 'UTC')::date THEN m.requests_used ELSE 0 END
 FROM channel_group_monitors m JOIN groups g ON g.id=m.group_id WHERE g.deleted_at IS NULL ORDER BY m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []service.GroupMonitor{}
	for rows.Next() {
		var p service.GroupMonitor
		var raw []byte
		if err := rows.Scan(&p.ID, &p.GroupID, &p.GroupName, &p.Platform, &raw, &p.NextProbeAt, &p.NextPelicanAt, &p.LeaseUntil, &p.RequestsUsed); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &p.Config); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	latest, err := r.db.QueryContext(ctx, `SELECT id,monitor_id,kind,model,status,detail-'html',created_at,finished_at
 FROM (SELECT id,monitor_id,kind,model,status,detail,created_at,finished_at,
 ROW_NUMBER() OVER(PARTITION BY monitor_id,kind,model ORDER BY id DESC) AS position FROM channel_group_monitor_runs) recent
 WHERE position<=30 ORDER BY monitor_id,id DESC`)
	if err != nil {
		return nil, err
	}
	defer latest.Close()
	index := map[int64]int{}
	for i := range items {
		items[i].LatestRuns = []service.GroupMonitorRun{}
		index[items[i].ID] = i
	}
	for latest.Next() {
		var run service.GroupMonitorRun
		var raw []byte
		if err = latest.Scan(&run.ID, &run.MonitorID, &run.Kind, &run.Model, &run.Status, &raw, &run.CreatedAt, &run.FinishedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &run.Detail); err != nil {
			return nil, err
		}
		if i, ok := index[run.MonitorID]; ok {
			items[i].LatestRuns = append(items[i].LatestRuns, run)
		}
	}
	return items, latest.Err()
}
func (r *groupMonitorRepository) SaveBatch(ctx context.Context, ids []int64, c service.GroupMonitorConfig) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		// Do not reset due times or leases when editing an existing plan.
		_, err = tx.ExecContext(ctx, `INSERT INTO channel_group_monitors(group_id,config,next_probe_at,next_pelican_at)
  VALUES($1,$2,NOW()+random()*INTERVAL '60 seconds',NOW()+random()*INTERVAL '60 seconds')
  ON CONFLICT(group_id) DO UPDATE SET config=EXCLUDED.config,updated_at=NOW()`, id, string(raw))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (r *groupMonitorRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM channel_group_monitors WHERE id=$1 AND (lease_until IS NULL OR lease_until<NOW())`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrGroupMonitorBusy
	}
	return err
}
func (r *groupMonitorRepository) Claim(ctx context.Context, id int64, kind, token string, manual bool) (*service.GroupMonitorClaim, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var p service.GroupMonitor
	var raw []byte
	var now time.Time
	var sameDay bool
	var used int
	err = tx.QueryRowContext(ctx, `SELECT id,group_id,config,next_probe_at,next_pelican_at,lease_until,requests_used,
 budget_day=(NOW() AT TIME ZONE 'UTC')::date,NOW() FROM channel_group_monitors WHERE id=$1 FOR UPDATE`, id).
		Scan(&p.ID, &p.GroupID, &raw, &p.NextProbeAt, &p.NextPelicanAt, &p.LeaseUntil, &used, &sameDay, &now)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGroupMonitorInvalid
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &p.Config); err != nil {
		return nil, err
	}
	if kind == "pelican" && strings.TrimSpace(p.Config.PelicanPrompt) == "" {
		return nil, service.ErrGroupMonitorInvalid
	}
	if !sameDay {
		used = 0
	}
	due := p.NextProbeAt
	interval := p.Config.ProbeInterval
	if kind == "pelican" {
		due = p.NextPelicanAt
		interval = p.Config.PelicanInterval
	}
	if (p.LeaseUntil != nil && p.LeaseUntil.After(now)) || (!manual && (!p.Config.Enabled || due.After(now) || (kind == "pelican" && !p.Config.PelicanEnabled))) || used+len(p.Config.Models) > p.Config.DailyLimit {
		return nil, service.ErrGroupMonitorBusy
	}
	// Expired workers cannot overwrite this terminal state: all writes also
	// require the current lease token. Never report an interrupted run as healthy.
	if _, err = tx.ExecContext(ctx, `UPDATE channel_group_monitor_runs SET status='failed',detail=detail || '{"error":"任务中断或超时"}'::jsonb,finished_at=NOW() WHERE monitor_id=$1 AND status='running'`, id); err != nil {
		return nil, err
	}
	column := "next_probe_at"
	if kind == "pelican" {
		column = "next_pelican_at"
	}
	_, err = tx.ExecContext(ctx, `UPDATE channel_group_monitors SET lease_token=$2,lease_until=NOW()+INTERVAL '20 minutes',requests_used=$3,budget_day=(NOW() AT TIME ZONE 'UTC')::date,`+column+`=NOW()+($4+random()*15)*INTERVAL '1 second' WHERE id=$1`, id, token, used+len(p.Config.Models), interval)
	if err != nil {
		return nil, err
	}
	claim := &service.GroupMonitorClaim{Monitor: p, Token: token, Runs: []service.GroupMonitorRun{}}
	for _, model := range p.Config.Models {
		run := service.GroupMonitorRun{MonitorID: id, Kind: kind, Model: model, Status: "running"}
		err = tx.QueryRowContext(ctx, `INSERT INTO channel_group_monitor_runs(monitor_id,lease_token,kind,model) VALUES($1,$2,$3,$4) RETURNING id,created_at`, id, token, kind, model).Scan(&run.ID, &run.CreatedAt)
		if err != nil {
			return nil, err
		}
		claim.Runs = append(claim.Runs, run)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return claim, nil
}
func (r *groupMonitorRepository) FinishRun(ctx context.Context, id int64, token string, run service.GroupMonitorRun) error {
	raw, err := json.Marshal(run.Detail)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE channel_group_monitor_runs SET status=$4,detail=$5,finished_at=NOW()
 WHERE id=$3 AND monitor_id=$1 AND lease_token=$2 AND status='running'
 AND EXISTS(SELECT 1 FROM channel_group_monitors WHERE id=$1 AND lease_token=$2 AND lease_until>NOW())`, id, token, run.ID, run.Status, string(raw))
	return err
}
func (r *groupMonitorRepository) Release(ctx context.Context, id int64, token string, keep int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE channel_group_monitors SET lease_token='',lease_until=NULL WHERE id=$1 AND lease_token=$2`, id, token)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM channel_group_monitor_runs WHERE monitor_id=$1 AND status<>'running' AND id NOT IN
 (SELECT id FROM channel_group_monitor_runs WHERE monitor_id=$1 ORDER BY id DESC LIMIT $2)
 AND id NOT IN (SELECT id FROM (SELECT id,ROW_NUMBER() OVER(PARTITION BY model ORDER BY id DESC) AS rank
 FROM channel_group_monitor_runs WHERE monitor_id=$1 AND kind='pelican') pelican WHERE rank<=3)`, id, keep)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *groupMonitorRepository) History(ctx context.Context, id int64, kind, model string, limit int) ([]service.GroupMonitorRun, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,monitor_id,kind,model,status,detail-'html',created_at,finished_at FROM channel_group_monitor_runs
 WHERE monitor_id=$1 AND ($2='' OR kind=$2) AND ($3='' OR model=$3) ORDER BY id DESC LIMIT $4`, id, kind, model, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []service.GroupMonitorRun{}
	for rows.Next() {
		var run service.GroupMonitorRun
		var raw []byte
		if err = rows.Scan(&run.ID, &run.MonitorID, &run.Kind, &run.Model, &run.Status, &raw, &run.CreatedAt, &run.FinishedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &run.Detail); err != nil {
			return nil, err
		}
		items = append(items, run)
	}
	return items, rows.Err()
}
func (r *groupMonitorRepository) Review(ctx context.Context, id, runID int64, review string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE channel_group_monitor_runs SET detail=jsonb_set(detail,'{review}',to_jsonb($3::text)) WHERE monitor_id=$1 AND id=$2 AND kind='pelican' AND status<>'running'`, id, runID, review)
	return err
}

func (r *groupMonitorRepository) Recover(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `UPDATE channel_group_monitor_runs SET status='failed',detail=detail || '{"error":"任务中断或超时"}'::jsonb,finished_at=NOW()
 WHERE status='running' AND created_at<NOW()-INTERVAL '20 minutes'`)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM channel_group_monitor_runs WHERE status<>'running' AND created_at<NOW()-INTERVAL '30 days'`)
	return err
}

func (r *groupMonitorRepository) GetRun(ctx context.Context, id, runID int64) (*service.GroupMonitorRun, error) {
	var run service.GroupMonitorRun
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT id,monitor_id,kind,model,status,detail,created_at,finished_at FROM channel_group_monitor_runs WHERE monitor_id=$1 AND id=$2`, id, runID).Scan(&run.ID, &run.MonitorID, &run.Kind, &run.Model, &run.Status, &raw, &run.CreatedAt, &run.FinishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGroupMonitorInvalid
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &run.Detail); err != nil {
		return nil, err
	}
	return &run, nil
}
