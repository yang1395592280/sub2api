package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

const walletHistoryEvents = `event_type IN ('admin_credit','admin_refund','checkin','checkin_bonus','checkin_clear','checkin_clear_refund','checkin_restore')`

func walletHistoryAmount(amount float64) string {
	rounded := decimal.NewFromFloat(amount).Round(2)
	if rounded.IsNegative() {
		return rounded.StringFixed(2)
	}
	return "+" + rounded.StringFixed(2)
}

// ListWalletBalanceHistory supplements legacy history without counting usage,
// frozen transfers or lottery records twice. Recharge totals exclude gifts.
func (r *userRepository) ListWalletBalanceHistory(ctx context.Context, userID int64, limit, offset int) ([]service.BalanceHistoryEntry, int64, float64, error) {
	var total int64
	var recharged float64
	rows, err := r.sql.QueryContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN event_type='admin_credit' THEN paid_delta ELSE 0 END),0) FROM wallet_transactions WHERE user_id=$1 AND `+walletHistoryEvents, userID)
	if err != nil {
		return nil, 0, 0, err
	}
	if rows.Next() {
		err = rows.Scan(&total, &recharged)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, 0, 0, err
	}
	items := []service.BalanceHistoryEntry{}
	if limit <= 0 {
		return items, total, recharged, nil
	}
	rows, err = r.sql.QueryContext(ctx, `SELECT id,event_type,operation_id,paid_delta,checkin_delta,liyu_delta,gift_delta,notes,created_at FROM wallet_transactions WHERE user_id=$1 AND `+walletHistoryEvents+` ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var e service.BalanceHistoryEntry
		var event string
		var delta service.WalletBalance
		if err := rows.Scan(&e.ID, &event, &e.Code, &delta.Paid, &delta.Checkin, &delta.Liyu, &delta.Gift, &e.Notes, &e.CreatedAt); err != nil {
			return nil, 0, 0, err
		}
		e.ReferenceID = fmt.Sprintf("wallet_transactions:%d", e.ID)
		e.ID = -3000000000 - e.ID
		e.Type = "wallet_activity"
		e.Status = service.StatusUsed
		e.Value = delta.Total()
		e.UsedBy = &userID
		at := e.CreatedAt
		e.UsedAt = &at
		labels := map[string]string{"admin_credit": "管理员充值与福利赠送", "admin_refund": "管理员余额退款", "checkin": "签到福利", "checkin_bonus": "签到翻倍", "checkin_clear": "签到福利清退", "checkin_clear_refund": "冻结签到福利清退", "checkin_restore": "签到福利复核恢复"}
		e.SourceLabel = labels[event]
		e.SourceDetail = fmt.Sprintf("充值 %s · 签到 %s · 礼遇 %s · 赠送 %s", walletHistoryAmount(delta.Paid), walletHistoryAmount(delta.Checkin), walletHistoryAmount(delta.Liyu), walletHistoryAmount(delta.Gift))
		if e.Notes != "" {
			e.SourceDetail += " · " + e.Notes
		}
		items = append(items, e)
	}
	return items, total, recharged, rows.Err()
}
