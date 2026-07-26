package service

import (
	"context"
	"fmt"
	"strconv"
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
		if isPriceGuardReasonForGroup(account.TempUnschedulableReason, group.ID) {
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
	if isPriceGuardReasonForGroup(account.TempUnschedulableReason, group.ID) {
		return repo.ClearTempUnschedulable(ctx, account.ID)
	}
	return nil
}

func isAccountBlockedByGroupUpstreamPriceGuard(account *Account, groupID *int64) bool {
	if account == nil || groupID == nil || *groupID <= 0 {
		return false
	}
	maxMultiplier, ok := accountGroupUpstreamPriceMaxMultiplier(account, *groupID)
	if !ok || maxMultiplier <= 0 {
		return false
	}
	if account.ChannelPrice == nil || *account.ChannelPrice <= 0 {
		return false
	}
	return *account.ChannelPrice > maxMultiplier
}

func (s *OpenAIGatewayService) isAccountBlockedByOpenAIGroupUpstreamPriceGuard(ctx context.Context, account *Account, groupID *int64) bool {
	if isAccountBlockedByGroupUpstreamPriceGuard(account, groupID) {
		return true
	}
	if account == nil || groupID == nil || *groupID <= 0 || account.ChannelPrice == nil || *account.ChannelPrice <= 0 {
		return false
	}
	effectiveGroup := openAIEffectiveGroupFromContext(ctx)
	if effectiveGroup == nil && s != nil && s.schedulerSnapshot != nil {
		effectiveGroup, _ = s.schedulerSnapshot.GetGroupByID(ctx, *groupID)
	}
	return effectiveGroup != nil && effectiveGroup.ID == *groupID && effectiveGroup.UpstreamPriceMaxMultiplier > 0 &&
		*account.ChannelPrice > effectiveGroup.UpstreamPriceMaxMultiplier
}

func accountGroupUpstreamPriceMaxMultiplier(account *Account, groupID int64) (float64, bool) {
	if account == nil || groupID <= 0 {
		return 0, false
	}
	for i := range account.AccountGroups {
		ag := account.AccountGroups[i]
		if ag.GroupID == groupID && ag.Group != nil {
			return ag.Group.UpstreamPriceMaxMultiplier, true
		}
	}
	for _, group := range account.Groups {
		if group != nil && group.ID == groupID {
			return group.UpstreamPriceMaxMultiplier, true
		}
	}
	return 0, false
}

func isPriceGuardReasonForGroup(reason string, groupID int64) bool {
	reason = strings.TrimSpace(reason)
	if !strings.HasPrefix(reason, UpstreamPriceGuardReasonPrefix) {
		return false
	}
	for _, field := range strings.Fields(strings.TrimSpace(strings.TrimPrefix(reason, UpstreamPriceGuardReasonPrefix))) {
		value, ok := strings.CutPrefix(field, "group_id=")
		if !ok {
			continue
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		return err == nil && parsed == groupID
	}
	return false
}
