package service

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestZenxiangLiyuValidatePrizesRequiresEnabledProbabilityTotal100(t *testing.T) {
	prizes := []ZenxiangLiyuPrize{
		{ID: 1, Name: "A", RewardAmount: 1, Probability: 40, Enabled: true},
		{ID: 2, Name: "B", RewardAmount: 3, Probability: 50, Enabled: true},
	}

	err := ValidateZenxiangLiyuPrizes(prizes)
	require.ErrorIs(t, err, ErrZenxiangLiyuInvalidProbabilityTotal)
}

func TestZenxiangLiyuValidatePrizesAcceptsConfiguredTiers(t *testing.T) {
	prizes := []ZenxiangLiyuPrize{
		{ID: 1, Name: "1", RewardAmount: 1, Probability: 70, Enabled: true},
		{ID: 2, Name: "3", RewardAmount: 3, Probability: 20, Enabled: true},
		{ID: 3, Name: "10", RewardAmount: 10, Probability: 10, Enabled: true},
	}

	require.NoError(t, ValidateZenxiangLiyuPrizes(prizes))
}

func TestPickZenxiangLiyuPrizeUsesProbabilityBoundaries(t *testing.T) {
	prizes := []ZenxiangLiyuPrize{
		{ID: 1, Name: "A", RewardAmount: 1, Probability: 70, Enabled: true},
		{ID: 2, Name: "B", RewardAmount: 3, Probability: 30, Enabled: true},
	}

	picked, err := PickZenxiangLiyuPrize(prizes, 69.9999)
	require.NoError(t, err)
	require.EqualValues(t, 1, picked.ID)

	picked, err = PickZenxiangLiyuPrize(prizes, 70.0000)
	require.NoError(t, err)
	require.EqualValues(t, 2, picked.ID)
}

func TestZenxiangLiyuSimulationComputesProfitAndUserDistribution(t *testing.T) {
	req := ZenxiangLiyuSimulationRequest{
		UserCount:      2,
		PlaysPerUser:   2,
		InitialBalance: 100,
		TicketAmount:   2,
		MinimumBalance: 10,
		DailyPlayLimit: 5,
		Prizes: []ZenxiangLiyuPrize{
			{ID: 1, Name: "1", RewardAmount: 1, Probability: 100, Enabled: true},
		},
	}
	svc := NewZenxiangLiyuService(nil, func() time.Time { return time.Unix(0, 0).UTC() }, rand.New(rand.NewSource(1)))

	result, err := svc.Simulate(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 4, result.TotalPlays)
	require.InDelta(t, 8, result.TotalRevenue, 0.000001)
	require.InDelta(t, 4, result.TotalExpense, 0.000001)
	require.InDelta(t, 4, result.NetProfit, 0.000001)
	require.Equal(t, 0, result.ProfitableUsers)
	require.Equal(t, 2, result.LosingUsers)
}

func TestZenxiangLiyuRecommendReturnsProbabilityTotal100(t *testing.T) {
	svc := NewZenxiangLiyuService(nil, func() time.Time { return time.Unix(0, 0).UTC() }, rand.New(rand.NewSource(1)))

	result, err := svc.Recommend(context.Background(), ZenxiangLiyuRecommendationRequest{
		TargetProfitRate: 0.05,
		TicketAmount:     2,
		Prizes: []ZenxiangLiyuPrize{
			{ID: 1, Name: "1", RewardAmount: 1, Enabled: true},
			{ID: 2, Name: "3", RewardAmount: 3, Enabled: true},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Plans)
	require.InDelta(t, 100, result.Plans[0].ProbabilityTotal, 0.000001)
	require.InDelta(t, 0.05, result.Plans[0].TheoryProfitRate, 0.000001)
}

func TestZenxiangLiyuRecommendUsesExactRewardForDuplicateTiers(t *testing.T) {
	svc := NewZenxiangLiyuService(nil, func() time.Time { return time.Unix(0, 0).UTC() }, rand.New(rand.NewSource(1)))

	result, err := svc.Recommend(context.Background(), ZenxiangLiyuRecommendationRequest{
		TargetProfitRate: 0.5,
		TicketAmount:     2,
		Prizes: []ZenxiangLiyuPrize{
			{ID: 1, Name: "A", RewardAmount: 1, Enabled: true},
			{ID: 2, Name: "B", RewardAmount: 1, Enabled: true},
			{ID: 3, Name: "C", RewardAmount: 3, Enabled: true},
		},
	})
	require.NoError(t, err)
	require.InDelta(t, 1, result.Plans[0].TheoryExpense, 0.000001)
	require.InDelta(t, 0.5, result.Plans[0].TheoryProfitRate, 0.000001)
}
