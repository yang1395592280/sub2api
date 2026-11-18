//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type gameExchangeRepoStub struct {
	GameCenterRepository
	balanceInput *ExchangeBalanceToPointsInput
	pointsInput  *ExchangePointsToBalanceInput
}

func (r *gameExchangeRepoStub) GetUserAssets(context.Context, int64) (*GameCenterAssets, error) {
	return &GameCenterAssets{Balance: 999, WelfareBalance: 5, Points: 100}, nil
}

func (r *gameExchangeRepoStub) ExchangeBalanceToPoints(_ context.Context, input ExchangeBalanceToPointsInput) (*GameCenterExchangeResult, error) {
	r.balanceInput = &input
	return &GameCenterExchangeResult{TargetPoints: input.TargetPoints}, nil
}

func (r *gameExchangeRepoStub) ExchangePointsToBalance(_ context.Context, input ExchangePointsToBalanceInput) (*GameCenterExchangeResult, error) {
	r.pointsInput = &input
	return &GameCenterExchangeResult{TargetAmount: input.TargetAmount}, nil
}

func TestGameCenterExchangeUsesConfiguredRatesAndMinimums(t *testing.T) {
	ctx := context.Background()
	repo := &gameExchangeRepoStub{}
	settings := &settingRepoStub{values: map[string]string{
		SettingKeyGameCenterExchangeBalanceToPointsEnabled: "true",
		SettingKeyGameCenterExchangePointsToBalanceEnabled: "true",
		SettingKeyGameCenterExchangeBalanceToPointsRate:    "37",
		SettingKeyGameCenterExchangePointsToBalanceRate:    "0.025",
		SettingKeyGameCenterExchangeMinBalanceAmount:       "0.1",
		SettingKeyGameCenterExchangeMinPointsAmount:        "4",
	}}
	svc := NewGameCenterService(repo, settings)
	view := svc.loadExchangeView(ctx)
	require.Equal(t, 0.1, view.MinBalanceAmount)
	require.Equal(t, int64(4), view.MinPointsAmount)
	_, err := svc.ExchangeBalanceToPoints(ctx, 7, 0.1)
	require.NoError(t, err)
	require.Equal(t, int64(4), repo.balanceInput.TargetPoints)
	_, err = svc.ExchangePointsToBalance(ctx, 7, 4)
	require.NoError(t, err)
	require.InDelta(t, 0.1, repo.pointsInput.TargetAmount, 1e-8)
	_, err = svc.ExchangeBalanceToPoints(ctx, 7, 0.09)
	require.ErrorIs(t, err, ErrGameCenterExchangeAmountTooSmall)
	_, err = svc.ExchangePointsToBalance(ctx, 7, 3)
	require.ErrorIs(t, err, ErrGameCenterExchangeAmountTooSmall)
	_, err = svc.ExchangeBalanceToPoints(ctx, 7, 6)
	require.ErrorIs(t, err, ErrGameCenterInsufficientBalance)
}

func TestGameCenterExchangeRejectsZeroPointCreditAndInvalidRates(t *testing.T) {
	ctx := context.Background()
	repo := &gameExchangeRepoStub{}
	settings := &settingRepoStub{values: map[string]string{
		SettingKeyGameCenterExchangeBalanceToPointsEnabled: "true",
		SettingKeyGameCenterExchangeBalanceToPointsRate:    "1",
	}}
	svc := NewGameCenterService(repo, settings)
	_, err := svc.ExchangeBalanceToPoints(ctx, 7, 0.01)
	require.ErrorIs(t, err, ErrGameCenterExchangeAmountTooSmall)
	require.Nil(t, repo.balanceInput)
	for _, rate := range []string{"0", "-1", "NaN", "+Inf"} {
		settings.values[SettingKeyGameCenterExchangeBalanceToPointsRate] = rate
		_, err := svc.ExchangeBalanceToPoints(ctx, 7, 1)
		require.ErrorIs(t, err, ErrGameCenterInvalidExchangeRate)
	}
}
