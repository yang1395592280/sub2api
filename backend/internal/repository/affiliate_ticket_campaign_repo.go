package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type affiliateTicketCampaignRepository struct {
	db *sql.DB
}

type affiliateTicketCampaignQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewAffiliateTicketCampaignRepository(db *sql.DB) service.AffiliateTicketCampaignRepository {
	return &affiliateTicketCampaignRepository{db: db}
}

func (r *affiliateTicketCampaignRepository) RecordRegistrationIP(ctx context.Context, userID int64, ip string) error {
	return r.RecordParticipation(ctx, userID, ip, "")
}

func (r *affiliateTicketCampaignRepository) RecordParticipation(ctx context.Context, userID int64, ip, deviceHash string) error {
	ip = canonicalCampaignIP(ip)
	if !campaignIPUsable(ip) {
		ip = ""
	}
	deviceHash = canonicalCampaignDeviceHash(deviceHash)
	if r == nil || r.db == nil || userID <= 0 || strings.TrimSpace(ip) == "" {
		if r == nil || r.db == nil || userID <= 0 || strings.TrimSpace(deviceHash) == "" {
			return nil
		}
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_affiliates
		SET registration_ip = CASE WHEN registration_ip = '' AND $1 <> '' THEN $1 ELSE registration_ip END,
		    campaign_device_hash = CASE WHEN campaign_device_hash = '' AND $2 <> '' THEN $2 ELSE campaign_device_hash END,
		    updated_at = NOW()
		WHERE user_id = $3`, ip, deviceHash, userID)
	return err
}

func (r *affiliateTicketCampaignRepository) GetEligibility(ctx context.Context, userID int64) (*service.AffiliateTicketCampaignEligibility, error) {
	if r == nil || r.db == nil || userID <= 0 {
		return emptyAffiliateTicketCampaignEligibility(), nil
	}
	return queryAffiliateTicketCampaignEligibility(ctx, r.db, userID)
}

func queryAffiliateTicketCampaignEligibility(ctx context.Context, queryer affiliateTicketCampaignQueryRower, userID int64) (*service.AffiliateTicketCampaignEligibility, error) {
	result := emptyAffiliateTicketCampaignEligibility()
	var status string
	err := queryer.QueryRowContext(ctx, `
		SELECT u.status,
		       u.balance::double precision,
		       EXISTS (
		           SELECT 1 FROM usage_logs ul
		           WHERE ul.user_id = u.id AND ul.actual_cost > 0
		       ),
		       COALESCE((
		           SELECT SUM(ul.actual_cost) FROM usage_logs ul
		           WHERE ul.user_id = u.id AND ul.actual_cost > 0
		       ), 0)::double precision
		FROM users u
		WHERE u.id = $1 AND u.deleted_at IS NULL`, userID).
		Scan(&status, &result.CurrentBalance, &result.HasUsageRecord, &result.HistoricalUsage)
	if err != nil {
		return nil, err
	}
	result.Eligible = affiliateTicketCampaignEligible(status, result.HasUsageRecord, result.HistoricalUsage, result.CurrentBalance)
	return result, nil
}

func affiliateTicketCampaignEligible(status string, hasUsageRecord bool, historicalUsage, currentBalance float64) bool {
	return status == service.StatusActive &&
		hasUsageRecord &&
		historicalUsage > service.AffiliateTicketCampaignUsageFloor &&
		currentBalance > service.AffiliateTicketCampaignBalanceFloor
}

func emptyAffiliateTicketCampaignEligibility() *service.AffiliateTicketCampaignEligibility {
	return &service.AffiliateTicketCampaignEligibility{
		HistoricalUsageThreshold: service.AffiliateTicketCampaignUsageFloor,
		BalanceThreshold:         service.AffiliateTicketCampaignBalanceFloor,
	}
}

func (r *affiliateTicketCampaignRepository) ProcessInviteRegistration(ctx context.Context, inviterID, inviteeID int64, inviteeIP, inviteeDeviceHash string, playDate time.Time) (_ *service.AffiliateTicketCampaignEvent, err error) {
	if r == nil || r.db == nil || inviterID <= 0 || inviteeID <= 0 || inviterID == inviteeID {
		return nil, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var existing service.AffiliateTicketCampaignEvent
	if scanErr := tx.QueryRowContext(ctx, campaignEventSelectSQL+` WHERE event_key = $1`, "invite_register:"+strconv.FormatInt(inviteeID, 10)).Scan(campaignEventScanArgs(&existing)...); scanErr == nil {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return &existing, nil
	} else if scanErr != sql.ErrNoRows {
		return nil, scanErr
	}

	var inviterIP, storedInviteeIP, inviterDeviceHash, storedInviteeDeviceHash, inviterStatus, inviteeStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT inviter.registration_ip, invitee.registration_ip,
		       inviter.campaign_device_hash, invitee.campaign_device_hash,
		       inviter_user.status, invitee_user.status
		FROM user_affiliates inviter
		JOIN user_affiliates invitee ON invitee.user_id = $2
		JOIN users inviter_user ON inviter_user.id = inviter.user_id
		JOIN users invitee_user ON invitee_user.id = invitee.user_id
		WHERE inviter.user_id = $1 AND invitee.inviter_id = $1
		FOR UPDATE OF inviter, invitee, inviter_user, invitee_user`, inviterID, inviteeID).
		Scan(&inviterIP, &storedInviteeIP, &inviterDeviceHash, &storedInviteeDeviceHash, &inviterStatus, &inviteeStatus)
	if errorsIsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(inviteeIP) == "" {
		inviteeIP = storedInviteeIP
	}
	if strings.TrimSpace(inviteeDeviceHash) == "" {
		inviteeDeviceHash = storedInviteeDeviceHash
	}
	inviterIP = canonicalCampaignIP(inviterIP)
	inviteeIP = canonicalCampaignIP(inviteeIP)
	inviterDeviceHash = canonicalCampaignDeviceHash(inviterDeviceHash)
	inviteeDeviceHash = canonicalCampaignDeviceHash(inviteeDeviceHash)
	if (storedInviteeIP == "" && inviteeIP != "") || (storedInviteeDeviceHash == "" && inviteeDeviceHash != "") {
		if _, err = tx.ExecContext(ctx, `UPDATE user_affiliates
			SET registration_ip = CASE WHEN registration_ip = '' THEN $1 ELSE registration_ip END,
			    campaign_device_hash = CASE WHEN campaign_device_hash = '' THEN $2 ELSE campaign_device_hash END,
			    updated_at = NOW() WHERE user_id = $3`, inviteeIP, inviteeDeviceHash, inviteeID); err != nil {
			return nil, err
		}
	}
	eligibility, eligibilityErr := queryAffiliateTicketCampaignEligibility(ctx, tx, inviterID)
	if eligibilityErr != nil {
		return nil, eligibilityErr
	}

	status := "granted"
	riskReason := ""
	// The campaign must never award a ticket when the trusted proxy chain did
	// not provide both registration addresses. A missing address is a
	// deployment/configuration risk, not proof of abuse, so keep registration
	// successful and record a non-rewarding event for auditability.
	if !campaignIPUsable(inviterIP) || !campaignIPUsable(inviteeIP) {
		status = "skipped"
		riskReason = "trusted registration IP unavailable"
	}
	sameIP, sameDevice := campaignRegistrationRisk(inviterIP, inviteeIP, inviterDeviceHash, inviteeDeviceHash)
	if sameDevice {
		status = "blocked"
		riskReason = "inviter and invitee share the same network and device"
		if _, err = tx.ExecContext(ctx, `
			UPDATE users SET status = $1, updated_at = NOW()
			WHERE id = $2 AND role = $3 AND deleted_at IS NULL`, service.StatusDisabled, inviteeID, service.RoleUser); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE user_affiliates
			SET risk_status = 'blocked', risk_reason = $1, updated_at = NOW()
			WHERE user_id = $2`, riskReason, inviteeID); err != nil {
			return nil, err
		}
		// A same-IP hit invalidates any still-unused campaign tickets earned by
		// this inviter today. Marking the source events as frozen keeps the
		// administrative audit trail aligned with the ticket pool.
		if _, err = tx.ExecContext(ctx, `
			UPDATE affiliate_ticket_campaign_batches b
			SET remaining_count = 0, updated_at = NOW()
			FROM affiliate_ticket_campaign_events e
			WHERE b.event_id = e.id
			  AND b.inviter_id = $1
			  AND e.play_date = $2
			  AND b.remaining_count > 0`, inviterID, playDate); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE affiliate_ticket_campaign_events e
			SET status = 'frozen', risk_reason = CASE WHEN e.risk_reason = '' THEN $3 ELSE e.risk_reason END,
			    updated_at = NOW()
			FROM affiliate_ticket_campaign_batches b
			WHERE b.event_id = e.id
			  AND b.inviter_id = $1
			  AND e.play_date = $2
			  AND e.status = 'granted'`, inviterID, playDate, "frozen after same-network and same-device invitation risk"); err != nil {
			return nil, err
		}
		var blockedCount int
		if err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM affiliate_ticket_campaign_events
			WHERE inviter_id = $1 AND status = 'blocked'`, inviterID).Scan(&blockedCount); err != nil {
			return nil, err
		}
		if blockedCount >= 1 {
			if _, err = tx.ExecContext(ctx, `
				UPDATE users SET status = $1, updated_at = NOW()
				WHERE id = $2 AND role = $3 AND deleted_at IS NULL`, service.StatusDisabled, inviterID, service.RoleUser); err != nil {
				return nil, err
			}
			if _, err = tx.ExecContext(ctx, `
				UPDATE user_affiliates
				SET risk_status = 'blocked', risk_reason = $1, updated_at = NOW()
				WHERE user_id = $2`, "repeated same-network and same-device invitation risk", inviterID); err != nil {
				return nil, err
			}
		}
	} else if sameIP {
		// Shared office/home egress IPs are common. Keep the event eligible when
		// the server-side session/device summaries differ, while leaving an
		// audit hint for administrators.
		riskReason = "identical registration IP with different device session"
	}
	if status == "granted" && !eligibility.Eligible {
		status = "skipped"
		riskReason = "inviter does not meet campaign eligibility"
	}
	if inviterStatus != service.StatusActive || inviteeStatus != service.StatusActive {
		status = "skipped"
		if riskReason == "" {
			riskReason = "inviter or invitee is not active"
		}
	}

	event := &service.AffiliateTicketCampaignEvent{}
	inviteeBonus := 0.0
	if status == "granted" {
		inviteeBonus = service.AffiliateTicketCampaignInviteeBonus
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO affiliate_ticket_campaign_events (
			event_key, event_type, inviter_id, invitee_id, play_date, amount, status,
			risk_reason, inviter_ip, invitee_ip
		) VALUES ($1, 'invite_register', $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, event_type, inviter_id, invitee_id, play_date, amount, ticket_count,
		          status, risk_reason, inviter_ip, invitee_ip, created_at`,
		"invite_register:"+strconv.FormatInt(inviteeID, 10), inviterID, inviteeID, playDate, inviteeBonus, status, riskReason, inviterIP, inviteeIP,
	).Scan(&event.ID, &event.EventType, &event.InviterID, &event.InviteeID, &event.PlayDate, &event.Amount,
		&event.TicketCount, &event.Status, &event.RiskReason, &event.InviterIP, &event.InviteeIP, &event.CreatedAt)
	if err != nil {
		return nil, err
	}
	if status == "granted" {
		result, updateErr := tx.ExecContext(ctx, `
			UPDATE users SET balance = balance + $1, updated_at = NOW()
			WHERE id = $2 AND status = $3 AND deleted_at IS NULL`, inviteeBonus, inviteeID, service.StatusActive)
		if updateErr != nil {
			return nil, updateErr
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
			if rowsErr != nil {
				return nil, rowsErr
			}
			return nil, fmt.Errorf("credit affiliate campaign invitee bonus: user %d is unavailable", inviteeID)
		}
		if err = r.grantForDailyEvent(ctx, tx, event, playDate, true); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return event, nil
}

func campaignIPUsable(raw string) bool {
	addr, err := netip.ParseAddr(canonicalCampaignIP(raw))
	if err != nil || !addr.IsValid() {
		return false
	}
	// Private/loopback addresses are commonly reverse-proxy or container
	// addresses. They are not reliable evidence for same-user detection.
	return !addr.IsPrivate() && !addr.IsLoopback() && !addr.IsUnspecified() && !addr.IsLinkLocalUnicast()
}

func canonicalCampaignIP(raw string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return addr.Unmap().String()
}

func canonicalCampaignDeviceHash(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > 128 {
		return raw[:128]
	}
	return raw
}

func campaignRegistrationRisk(inviterIP, inviteeIP, inviterDeviceHash, inviteeDeviceHash string) (sameIP, sameDevice bool) {
	inviterIP = canonicalCampaignIP(inviterIP)
	inviteeIP = canonicalCampaignIP(inviteeIP)
	sameIP = campaignIPUsable(inviterIP) && campaignIPUsable(inviteeIP) && inviterIP == inviteeIP
	if !sameIP {
		return false, false
	}
	inviterDeviceHash = canonicalCampaignDeviceHash(inviterDeviceHash)
	inviteeDeviceHash = canonicalCampaignDeviceHash(inviteeDeviceHash)
	sameDevice = inviterDeviceHash != "" && inviteeDeviceHash != "" && inviterDeviceHash == inviteeDeviceHash
	return sameIP, sameDevice
}

func (r *affiliateTicketCampaignRepository) ProcessInviteRecharge(ctx context.Context, inviteeID, orderID int64, amount float64, playDate time.Time) (_ *service.AffiliateTicketCampaignEvent, err error) {
	if r == nil || r.db == nil || inviteeID <= 0 || orderID <= 0 || amount < service.AffiliateTicketCampaignRechargeFloor {
		return nil, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var inviterID int64
	var riskStatus, inviterIP, inviteeIP, inviterStatus, inviteeStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT invitee.inviter_id, invitee.risk_status, inviter.registration_ip, invitee.registration_ip,
		       inviter_user.status, invitee_user.status
		FROM user_affiliates invitee
		LEFT JOIN user_affiliates inviter ON inviter.user_id = invitee.inviter_id
		JOIN users inviter_user ON inviter_user.id = invitee.inviter_id
		JOIN users invitee_user ON invitee_user.id = invitee.user_id
		WHERE invitee.user_id = $1
		FOR SHARE`, inviteeID).Scan(&inviterID, &riskStatus, &inviterIP, &inviteeIP, &inviterStatus, &inviteeStatus)
	if errorsIsNoRows(err) || inviterID <= 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	eventKey := "invite_recharge:" + strconv.FormatInt(inviteeID, 10)
	var existing service.AffiliateTicketCampaignEvent
	if scanErr := tx.QueryRowContext(ctx, campaignEventSelectSQL+` WHERE event_key = $1`, eventKey).Scan(campaignEventScanArgs(&existing)...); scanErr == nil {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return &existing, nil
	} else if scanErr != sql.ErrNoRows {
		return nil, scanErr
	}

	status := "granted"
	riskReason := ""
	eligibility, eligibilityErr := queryAffiliateTicketCampaignEligibility(ctx, tx, inviterID)
	if eligibilityErr != nil {
		return nil, eligibilityErr
	}
	if !eligibility.Eligible {
		status = "skipped"
		riskReason = "inviter does not meet campaign eligibility"
	}
	var hasPriorRecharge bool
	if err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM payment_orders prior_order
			WHERE prior_order.user_id = $1
			  AND prior_order.id <> $2
			  AND prior_order.order_type = 'balance'
			  AND prior_order.paid_at IS NOT NULL
			  AND prior_order.status <> 'PENDING'
		)`, inviteeID, orderID).Scan(&hasPriorRecharge); err != nil {
		return nil, err
	}
	if hasPriorRecharge {
		status = "skipped"
		riskReason = "invitee has already completed an earlier recharge"
	}
	if riskStatus != "" && riskStatus != "clear" {
		status = "blocked"
		riskReason = "invite relationship is under risk control"
	}
	if inviterStatus != service.StatusActive || inviteeStatus != service.StatusActive {
		status = "blocked"
		riskReason = "inviter or invitee is not active"
	}
	event := &service.AffiliateTicketCampaignEvent{}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO affiliate_ticket_campaign_events (
			event_key, event_type, inviter_id, invitee_id, order_id, play_date,
			amount, status, risk_reason, inviter_ip, invitee_ip
		) VALUES ($1, 'invite_recharge', $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, event_type, inviter_id, invitee_id, order_id, play_date, amount,
		          ticket_count, status, risk_reason, inviter_ip, invitee_ip, created_at`,
		eventKey, inviterID, inviteeID, orderID, playDate, amount, status, riskReason, inviterIP, inviteeIP,
	).Scan(&event.ID, &event.EventType, &event.InviterID, &event.InviteeID, &event.OrderID, &event.PlayDate,
		&event.Amount, &event.TicketCount, &event.Status, &event.RiskReason, &event.InviterIP, &event.InviteeIP, &event.CreatedAt)
	if err != nil {
		return nil, err
	}
	if status == "granted" {
		if err = r.grantForDailyEvent(ctx, tx, event, playDate, false); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return event, nil
}

func (r *affiliateTicketCampaignRepository) grantForDailyEvent(ctx context.Context, tx *sql.Tx, event *service.AffiliateTicketCampaignEvent, playDate time.Time, registration bool) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_ticket_campaign_daily (inviter_id, play_date)
		VALUES ($1, $2) ON CONFLICT (inviter_id, play_date) DO NOTHING`, event.InviterID, playDate); err != nil {
		return err
	}
	var registered, recharge, currentTickets int
	if err := tx.QueryRowContext(ctx, `
		SELECT registered_count, recharge_count, ticket_count
		FROM affiliate_ticket_campaign_daily
		WHERE inviter_id = $1 AND play_date = $2 FOR UPDATE`, event.InviterID, playDate).
		Scan(&registered, &recharge, &currentTickets); err != nil {
		return err
	}
	if registration {
		registered++
	} else {
		recharge++
	}
	target := affiliateTicketCampaignTarget(registered, recharge)
	delta := affiliateTicketCampaignGrantDelta(registered, recharge, currentTickets)
	if delta > 0 {
		event.TicketCount = delta
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO affiliate_ticket_campaign_batches (
				inviter_id, event_id, granted_count, remaining_count, expires_at
			) VALUES ($1, $2, $3, $3,
				(((CAST($4 AS DATE) + $5::integer)::timestamp) AT TIME ZONE 'Asia/Shanghai'))`,
			event.InviterID, event.ID, delta, playDate.Format("2006-01-02"), service.AffiliateTicketCampaignRetentionDays); err != nil {
			return err
		}
	}
	status := event.Status
	if delta == 0 && target >= service.AffiliateTicketCampaignDailyCap {
		status = "skipped"
		event.Status = status
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE affiliate_ticket_campaign_daily
		SET registered_count = $1, recharge_count = $2, ticket_count = $3, updated_at = NOW()
		WHERE inviter_id = $4 AND play_date = $5`, registered, recharge, currentTickets+delta, event.InviterID, playDate)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE affiliate_ticket_campaign_events
		SET ticket_count = $1, status = $2, updated_at = NOW()
		WHERE id = $3`, event.TicketCount, status, event.ID)
	return err
}

func affiliateTicketCampaignTarget(registered, recharge int) int {
	target := maxInt(0, registered)/service.AffiliateTicketCampaignRegisterPair + maxInt(0, recharge)
	if target > service.AffiliateTicketCampaignDailyCap {
		return service.AffiliateTicketCampaignDailyCap
	}
	return target
}

func affiliateTicketCampaignGrantDelta(registered, recharge, currentTickets int) int {
	return maxInt(0, affiliateTicketCampaignTarget(registered, recharge)-maxInt(0, currentTickets))
}

func (r *affiliateTicketCampaignRepository) GetDaily(ctx context.Context, inviterID int64, playDate time.Time) (*service.AffiliateTicketCampaignDaily, error) {
	result := &service.AffiliateTicketCampaignDaily{PlayDate: playDate, DailyCap: service.AffiliateTicketCampaignDailyCap}
	err := r.db.QueryRowContext(ctx, `
		SELECT registered_count, recharge_count, ticket_count
		FROM affiliate_ticket_campaign_daily
		WHERE inviter_id = $1 AND play_date = $2`, inviterID, playDate).
		Scan(&result.RegisteredCount, &result.RechargeCount, &result.TicketCount)
	if err == sql.ErrNoRows {
		result.TicketsRemaining = result.DailyCap
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	result.TicketsRemaining = maxInt(0, result.DailyCap-result.TicketCount)
	return result, nil
}

func (r *affiliateTicketCampaignRepository) ListInvitees(ctx context.Context, inviterID int64, limit int) ([]service.AffiliateTicketCampaignInvitee, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.email, u.username, u.created_at,
		       u.status, ua.risk_status,
		       EXISTS (
				SELECT 1 FROM affiliate_ticket_campaign_events e
				WHERE e.invitee_id = u.id AND e.event_type = 'invite_recharge'
				  AND e.status IN ('granted', 'skipped')
			),
		       COALESCE((SELECT CASE WHEN SUM(e.ticket_count) > 0 THEN 'granted' ELSE MAX(e.status) END
				FROM affiliate_ticket_campaign_events e WHERE e.invitee_id = u.id), 'pending')
		FROM user_affiliates ua
		JOIN users u ON u.id = ua.user_id
		WHERE ua.inviter_id = $1
		ORDER BY u.created_at DESC, u.id DESC LIMIT $2`, inviterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]service.AffiliateTicketCampaignInvitee, 0)
	for rows.Next() {
		var item service.AffiliateTicketCampaignInvitee
		var userStatus, riskStatus string
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.RegisteredAt, &userStatus, &riskStatus, &item.RechargeQualified, &item.TicketStatus); err != nil {
			return nil, err
		}
		item.RegistrationStatus = userStatus
		item.RiskStatus = riskStatus
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *affiliateTicketCampaignRepository) ListEvents(ctx context.Context, filter service.AffiliateTicketCampaignEventFilter) ([]service.AffiliateTicketCampaignEvent, int, error) {
	page, pageSize := filter.Page, filter.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	where := []string{"1=1"}
	args := []any{}
	add := func(expr string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		where = append(where, fmt.Sprintf("(inviter.email ILIKE $%d OR invitee.email ILIKE $%d OR e.inviter_ip ILIKE $%d OR e.invitee_ip ILIKE $%d OR e.inviter_id::text ILIKE $%d OR e.invitee_id::text ILIKE $%d)", len(args), len(args), len(args), len(args), len(args), len(args)))
	}
	if filter.Status != "" {
		add("e.status = $%d", filter.Status)
	}
	if filter.EventType != "" {
		add("e.event_type = $%d", filter.EventType)
	}
	if filter.StartAt != nil {
		add("e.created_at >= $%d", *filter.StartAt)
	}
	if filter.EndAt != nil {
		add("e.created_at < $%d", *filter.EndAt)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	countSQL := "SELECT COUNT(*) FROM affiliate_ticket_campaign_events e JOIN users inviter ON inviter.id = e.inviter_id JOIN users invitee ON invitee.id = e.invitee_id WHERE " + whereSQL
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, pageSize, (page-1)*pageSize)
	listSQL := campaignEventSelectSQL + ` WHERE ` + whereSQL + fmt.Sprintf(" ORDER BY e.created_at DESC, e.id DESC LIMIT $%d OFFSET $%d", len(listArgs)-1, len(listArgs))
	rows, err := r.db.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]service.AffiliateTicketCampaignEvent, 0, pageSize)
	for rows.Next() {
		var item service.AffiliateTicketCampaignEvent
		if err := rows.Scan(campaignEventScanArgs(&item)...); err != nil {
			return nil, 0, err
		}
		if item.InviterEmail != "" {
			item.InviterEmail = maskCampaignEmail(item.InviterEmail)
		}
		if item.InviteeEmail != "" {
			item.InviteeEmail = maskCampaignEmail(item.InviteeEmail)
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

const campaignEventSelectSQL = `
	SELECT e.id, e.event_type, e.inviter_id, inviter.email, e.invitee_id, invitee.email,
	       e.order_id, e.play_date, e.amount, e.ticket_count, e.status, e.risk_reason,
	       e.inviter_ip, e.invitee_ip, e.created_at
	FROM affiliate_ticket_campaign_events e
	JOIN users inviter ON inviter.id = e.inviter_id
	JOIN users invitee ON invitee.id = e.invitee_id`

func campaignEventScanArgs(event *service.AffiliateTicketCampaignEvent) []any {
	return []any{&event.ID, &event.EventType, &event.InviterID, &event.InviterEmail, &event.InviteeID, &event.InviteeEmail,
		&event.OrderID, &event.PlayDate, &event.Amount, &event.TicketCount, &event.Status, &event.RiskReason,
		&event.InviterIP, &event.InviteeIP, &event.CreatedAt}
}

func errorsIsNoRows(err error) bool { return err == sql.ErrNoRows }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maskCampaignEmail(email string) string {
	parts := strings.SplitN(strings.TrimSpace(email), "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}
	local := []rune(parts[0])
	return string(local[0]) + "***@" + parts[1]
}
