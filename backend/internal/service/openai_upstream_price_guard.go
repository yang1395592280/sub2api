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

	actual := account.EffectiveChannelPrice()
	updates := map[string]any{
		"upstream_price_guard_group_id":          group.ID,
		"upstream_price_guard_max_multiplier":    group.UpstreamPriceMaxMultiplier,
		"upstream_price_guard_actual_multiplier": actual,
		"upstream_price_guard_checked_at":        now.UTC().Format(time.RFC3339),
		"upstream_price_guard_error":             "",
	}

	if group.UpstreamPriceMaxMultiplier <= 0 {
		updates["upstream_price_guard_status"] = "ok"
		return repo.UpdateExtra(ctx, account.ID, updates)
	}

	if account.ChannelPrice == nil || actual <= 0 {
		updates["upstream_price_guard_status"] = "unsupported"
		updates["upstream_price_guard_error"] = "missing upstream effective multiplier"
		return repo.UpdateExtra(ctx, account.ID, updates)
	}

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
