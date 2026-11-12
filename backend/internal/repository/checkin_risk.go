package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const checkinRiskColumns = `id,user_id,status,reason,evidence,observe_until,cleared_amount,created_at,decided_at`

func scanCheckinRisk(row interface{ Scan(...any) error }) (*service.CheckinRiskCase, error) {
	c := new(service.CheckinRiskCase)
	err := row.Scan(&c.ID, &c.UserID, &c.Status, &c.Reason, &c.Evidence, &c.ObserveUntil, &c.ClearedAmount, &c.CreatedAt, &c.DecidedAt)
	return c, err
}

func checkCheckinAllowed(ctx context.Context, tx *sql.Tx, userID int64) error {
	var blocked bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM checkin_risk_cases WHERE user_id=$1 AND status='cleared')`, userID).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return service.ErrCheckinSuspended
	}
	return nil
}

func (r *checkinRepository) GetCheckinRisk(ctx context.Context, userID int64) (*service.CheckinRiskCase, error) {
	c, err := scanCheckinRisk(r.db.QueryRowContext(ctx, `SELECT `+checkinRiskColumns+` FROM checkin_risk_cases WHERE user_id=$1 AND status IN ('observing','cleared') ORDER BY id DESC LIMIT 1`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func (r *checkinRepository) ConfirmCheckinRisk(ctx context.Context, userID, actorID int64, reason, evidence string) (*service.CheckinRiskCase, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = lockWallet(ctx, tx, userID); err != nil {
		return nil, err
	}
	c, err := scanCheckinRisk(tx.QueryRowContext(ctx, `INSERT INTO checkin_risk_cases(user_id,reason,evidence,confirmed_by) VALUES($1,$2,$3,NULLIF($4,0))
		ON CONFLICT(user_id) WHERE status IN ('observing','cleared') DO UPDATE SET user_id=EXCLUDED.user_id RETURNING `+checkinRiskColumns, userID, reason, evidence, actorID))
	if err != nil {
		return nil, err
	}
	return c, tx.Commit()
}

func (r *checkinRepository) ResolveCheckinRisk(ctx context.Context, caseID, actorID int64, action string) (int64, error) {
	var userID int64
	if err := r.db.QueryRowContext(ctx, `SELECT user_id FROM checkin_risk_cases WHERE id=$1`, caseID).Scan(&userID); err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err = lockWallet(ctx, tx, userID); err != nil {
		return 0, err
	}
	c, err := scanCheckinRisk(tx.QueryRowContext(ctx, `SELECT `+checkinRiskColumns+` FROM checkin_risk_cases WHERE id=$1 FOR UPDATE`, caseID))
	if err != nil {
		return 0, err
	}
	status := "revoked"
	if c.Status == "cleared" {
		if action != "restore" {
			return 0, errors.New("cleared risk requires restore")
		}
		if _, err = applyWalletDelta(ctx, tx, userID, service.WalletBalance{Checkin: c.ClearedAmount}, "checkin_restore", fmt.Sprint(caseID), 0, &actorID, "管理员复核恢复签到福利"); err != nil {
			return 0, err
		}
		status = "restored"
	} else if c.Status != "observing" {
		return userID, nil
	} else if action != "revoke" {
		return 0, errors.New("observing risk requires revoke")
	}
	_, err = tx.ExecContext(ctx, `UPDATE checkin_risk_cases SET status=$1,decided_at=NOW(),resolved_by=NULLIF($3,0) WHERE id=$2`, status, caseID, actorID)
	if err != nil {
		return 0, err
	}
	return userID, tx.Commit()
}

func (r *checkinRepository) ListCheckinRisks(ctx context.Context, page, size int) ([]service.CheckinRiskCase, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM checkin_risk_cases`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+checkinRiskColumns+` FROM checkin_risk_cases ORDER BY id DESC LIMIT $1 OFFSET $2`, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []service.CheckinRiskCase{}
	for rows.Next() {
		c, err := scanCheckinRisk(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *c)
	}
	return items, total, rows.Err()
}

func (r *checkinRepository) ProcessDueCheckinRisks(ctx context.Context) ([]int64, error) {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM checkin_page_tokens WHERE expires_at < NOW()`); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,user_id FROM checkin_risk_cases WHERE status='observing' AND observe_until<=NOW() ORDER BY id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	type dueCase struct{ id, userID int64 }
	cases := []dueCase{}
	for rows.Next() {
		var c dueCase
		if err := rows.Scan(&c.id, &c.userID); err != nil {
			rows.Close()
			return nil, err
		}
		cases = append(cases, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	processed := []int64{}
	for _, c := range cases {
		if err := r.clearCheckinRisk(ctx, c.id, c.userID); err != nil {
			return processed, err
		}
		processed = append(processed, c.userID)
	}
	return processed, nil
}

func (r *checkinRepository) clearCheckinRisk(ctx context.Context, id, userID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	w, err := lockWallet(ctx, tx, userID)
	if errors.Is(err, service.ErrUserNotFound) {
		// A soft-deleted account must not block all later cases in the sweep.
		if _, err = tx.ExecContext(ctx, `UPDATE checkin_risk_cases SET status='revoked',decided_at=NOW() WHERE id=$1 AND status='observing'`, id); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	var claimed int64
	err = tx.QueryRowContext(ctx, `UPDATE checkin_risk_cases SET status='cleared',cleared_amount=$1,decided_at=NOW()
		WHERE id=$2 AND status='observing' AND observe_until<=NOW() RETURNING id`, math.Max(w.Checkin, 0), id).Scan(&claimed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// Preserve negative activity debt: clearing it would reward an abusive account.
	if _, err = applyWalletDelta(ctx, tx, userID, service.WalletBalance{Checkin: -math.Max(w.Checkin, 0)}, "checkin_clear", fmt.Sprint(id), 0, nil, "违规自动签到福利清退"); err != nil {
		return err
	}
	// Pending holds keep their original split. Only unspent check-in refunds
	// are revoked on release, so already consumed funds are never clawed back.
	if _, err = tx.ExecContext(ctx, `UPDATE wallet_image_holds SET checkin_risk_case_id=$1 WHERE user_id=$2 AND settled=FALSE AND checkin>0`, id, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *checkinRepository) RecordCheckinSecurityEvent(ctx context.Context, userID int64, event, ip, ua string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO checkin_security_events(user_id,event_type,client_ip,user_agent) VALUES($1,$2,$3,$4)`, userID, event, ip, ua)
	if err != nil {
		return err
	}
	// Require independent evidence across multiple days. Fixed schedules, IPs,
	// user agents, and repeated HTTP requests alone never confiscate funds.
	var days, trap, captchaFailures int
	err = r.db.QueryRowContext(ctx, `SELECT
		COUNT(DISTINCT (created_at AT TIME ZONE 'Asia/Shanghai')::date) FILTER(WHERE event_type='page_bypass'),
		COUNT(*) FILTER(WHERE event_type='honeypot'),
		COUNT(*) FILTER(WHERE event_type='captcha_failure')
		FROM checkin_security_events WHERE user_id=$1 AND created_at>=NOW()-INTERVAL '14 days'`, userID).Scan(&days, &trap, &captchaFailures)
	if err != nil {
		return err
	}
	if days >= 3 && trap >= 3 && captchaFailures >= 3 {
		_, err = r.ConfirmCheckinRisk(ctx, userID, 0, "连续绕过签到页面验证并触发自动化领取检测", fmt.Sprintf("14日内：跨%d天页面验证缺失，%d次隐藏字段提交，%d次人机验证失败", days, trap, captchaFailures))
	}
	return err
}

func (r *checkinRepository) CreateCheckinPageToken(ctx context.Context, userID int64, hash string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO checkin_page_tokens(user_id,token_hash) VALUES($1,$2)`, userID, hash)
	return err
}

func (r *checkinRepository) ConsumeCheckinPageToken(ctx context.Context, userID int64, hash string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE checkin_page_tokens SET consumed=TRUE WHERE token_hash=$1 AND user_id=$2 AND expires_at>NOW() AND consumed=FALSE`, hash, userID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
