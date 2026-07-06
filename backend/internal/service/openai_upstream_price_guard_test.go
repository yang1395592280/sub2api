package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type upstreamPriceGuardRepoStub struct {
	AccountRepository
	setUntil    time.Time
	setReason   string
	setCalled   bool
	clearCalled bool
	updates     map[string]any
}

func (r *upstreamPriceGuardRepoStub) SetTempUnschedulable(_ context.Context, _ int64, until time.Time, reason string) error {
	r.setCalled = true
	r.setUntil = until
	r.setReason = reason
	return nil
}

func (r *upstreamPriceGuardRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	r.clearCalled = true
	return nil
}

func (r *upstreamPriceGuardRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updates = updates
	return nil
}

func TestApplyGroupUpstreamPriceGuard_BlocksWhenPriceExceedsLimit(t *testing.T) {
	price := 0.12
	account := &Account{ID: 7, ChannelPrice: &price}
	group := Group{ID: 3, UpstreamPriceMaxMultiplier: 0.08}
	now := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	repo := &upstreamPriceGuardRepoStub{}

	err := ApplyGroupUpstreamPriceGuard(context.Background(), repo, account, group, now)

	require.NoError(t, err)
	require.True(t, repo.setCalled)
	require.Equal(t, now.Add(24*time.Hour), repo.setUntil)
	require.Contains(t, repo.setReason, UpstreamPriceGuardReasonPrefix)
	require.Equal(t, "blocked", repo.updates["upstream_price_guard_status"])
	require.Equal(t, int64(3), repo.updates["upstream_price_guard_group_id"])
}

func TestApplyGroupUpstreamPriceGuard_ClearsOnlyOwnReasonWhenPriceRecovers(t *testing.T) {
	price := 0.06
	account := &Account{ID: 7, ChannelPrice: &price, TempUnschedulableReason: UpstreamPriceGuardReasonPrefix + " group_id=3"}
	group := Group{ID: 3, UpstreamPriceMaxMultiplier: 0.08}
	repo := &upstreamPriceGuardRepoStub{}

	err := ApplyGroupUpstreamPriceGuard(context.Background(), repo, account, group, time.Now())

	require.NoError(t, err)
	require.True(t, repo.clearCalled)
	require.Equal(t, "ok", repo.updates["upstream_price_guard_status"])
}

func TestApplyGroupUpstreamPriceGuard_DoesNotClearOtherTempReason(t *testing.T) {
	price := 0.06
	account := &Account{ID: 7, ChannelPrice: &price, TempUnschedulableReason: "token refresh retry exhausted"}
	group := Group{ID: 3, UpstreamPriceMaxMultiplier: 0.08}
	repo := &upstreamPriceGuardRepoStub{}

	err := ApplyGroupUpstreamPriceGuard(context.Background(), repo, account, group, time.Now())

	require.NoError(t, err)
	require.False(t, repo.clearCalled)
}

func TestApplyGroupUpstreamPriceGuard_ClearsOwnReasonWhenGuardDisabled(t *testing.T) {
	price := 0.06
	account := &Account{ID: 7, ChannelPrice: &price, TempUnschedulableReason: UpstreamPriceGuardReasonPrefix + " group_id=3"}
	group := Group{ID: 3, UpstreamPriceMaxMultiplier: 0}
	repo := &upstreamPriceGuardRepoStub{}

	err := ApplyGroupUpstreamPriceGuard(context.Background(), repo, account, group, time.Now())

	require.NoError(t, err)
	require.True(t, repo.clearCalled)
	require.Equal(t, "ok", repo.updates["upstream_price_guard_status"])
}

func TestApplyGroupUpstreamPriceGuard_TreatsNonPositiveChannelPriceAsUnsupported(t *testing.T) {
	price := 0.0
	account := &Account{ID: 7, ChannelPrice: &price}
	group := Group{ID: 3, UpstreamPriceMaxMultiplier: 0.08}
	repo := &upstreamPriceGuardRepoStub{}

	err := ApplyGroupUpstreamPriceGuard(context.Background(), repo, account, group, time.Now())

	require.NoError(t, err)
	require.True(t, repo.updates != nil)
	require.Equal(t, "unsupported", repo.updates["upstream_price_guard_status"])
	require.NotEmpty(t, repo.updates["upstream_price_guard_error"])
	require.False(t, repo.setCalled)
	require.False(t, repo.clearCalled)
}
