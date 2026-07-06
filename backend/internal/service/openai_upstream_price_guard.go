package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	UpstreamPriceGuardReasonPrefix = "upstream_price_guard:"
	upstreamPriceGuardBlockTTL     = 24 * time.Hour
)

func ApplyGroupUpstreamPriceGuard(ctx context.Context, repo AccountRepository, account *Account, group Group, now time.Time) error {
	if repo == nil || account == nil || account.ID <= 0 {
		return nil
	}

	updates := map[string]any{
		"upstream_price_guard_group_id":       group.ID,
		"upstream_price_guard_max_multiplier": group.UpstreamPriceMaxMultiplier,
		"upstream_price_guard_checked_at":     now.UTC().Format(time.RFC3339),
		"upstream_price_guard_error":          "",
	}

	if group.UpstreamPriceMaxMultiplier <= 0 {
		updates["upstream_price_guard_status"] = "ok"
		if err := repo.UpdateExtra(ctx, account.ID, updates); err != nil {
			return err
		}
		if strings.HasPrefix(strings.TrimSpace(account.TempUnschedulableReason), UpstreamPriceGuardReasonPrefix) {
			return repo.ClearTempUnschedulable(ctx, account.ID)
		}
		return nil
	}

	if account.ChannelPrice == nil || *account.ChannelPrice <= 0 {
		updates["upstream_price_guard_actual_multiplier"] = nil
		updates["upstream_price_guard_status"] = "unsupported"
		updates["upstream_price_guard_error"] = "missing or invalid upstream channel price"
		return repo.UpdateExtra(ctx, account.ID, updates)
	}

	actual := *account.ChannelPrice
	updates["upstream_price_guard_actual_multiplier"] = actual

	if actual > group.UpstreamPriceMaxMultiplier {
		updates["upstream_price_guard_status"] = "blocked"
		if err := repo.UpdateExtra(ctx, account.ID, updates); err != nil {
			return err
		}
		reason := fmt.Sprintf("%s group_id=%d actual=%.6f max=%.6f", UpstreamPriceGuardReasonPrefix, group.ID, actual, group.UpstreamPriceMaxMultiplier)
		return repo.SetTempUnschedulable(ctx, account.ID, now.Add(upstreamPriceGuardBlockTTL), reason)
	}

	updates["upstream_price_guard_status"] = "ok"
	if err := repo.UpdateExtra(ctx, account.ID, updates); err != nil {
		return err
	}
	if strings.HasPrefix(strings.TrimSpace(account.TempUnschedulableReason), UpstreamPriceGuardReasonPrefix) {
		return repo.ClearTempUnschedulable(ctx, account.ID)
	}
	return nil
}
