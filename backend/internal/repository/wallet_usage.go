package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// Read the committed settlement, never estimate its split from today's wallet.
// Missing entries stay nil: historical or not-yet-settled usage is unclassified.
func (r *usageLogRepository) hydrateWalletUsage(ctx context.Context, logs []service.UsageLog) error {
	if r.db == nil || len(logs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(logs))
	byID := make(map[int64]int, len(logs))
	for i := range logs {
		ids = append(ids, logs[i].ID)
		byID[logs[i].ID] = i
	}
	rows, err := r.db.QueryContext(ctx, `SELECT u.id,-w.paid_delta,-w.checkin_delta,-w.liyu_delta,-w.gift_delta
		FROM usage_logs u JOIN wallet_transactions w ON w.user_id=u.user_id AND w.api_key_id=u.api_key_id
		AND w.operation_id=u.request_id AND w.event_type='usage' WHERE u.id=ANY($1)`, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("load wallet usage breakdown: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var b service.WalletUsageBreakdown
		if err := rows.Scan(&id, &b.PaidCost, &b.CheckinCost, &b.LiyuCost, &b.GiftCost); err != nil {
			return err
		}
		b.WelfareCost = service.QuantizeUsageBillingAmount(b.CheckinCost + b.LiyuCost + b.GiftCost)
		if i, ok := byID[id]; ok {
			logs[i].WalletUsage = &b
		}
	}
	return rows.Err()
}
