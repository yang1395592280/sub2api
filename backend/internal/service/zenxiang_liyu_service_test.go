package service

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type zenxiangLiyuServiceTestRepository struct {
	mu sync.Mutex

	settings         ZenxiangLiyuSettings
	prizes           []ZenxiangLiyuPrize
	granted          bool
	balance          float64
	todayUsageAmount float64
	freePlayUsed     bool
	giftedTickets    int
	ticketBalance    int
	grantCalls       int
	giftCalls        int
	deleteCalls      int
	bulkSaveCalls    int
	countUserPlays   int
	playCalls        int
	lastPlayCommand  ZenxiangLiyuPlayCommand
}

func (r *zenxiangLiyuServiceTestRepository) GetSettings(context.Context) (*ZenxiangLiyuSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	settings := r.settings
	return &settings, nil
}

func (r *zenxiangLiyuServiceTestRepository) UpdateSettings(_ context.Context, settings ZenxiangLiyuSettings) (*ZenxiangLiyuSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings = settings
	return &settings, nil
}

func (r *zenxiangLiyuServiceTestRepository) ListPrizes(context.Context) ([]ZenxiangLiyuPrize, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ZenxiangLiyuPrize(nil), r.prizes...), nil
}

func (r *zenxiangLiyuServiceTestRepository) SavePrize(_ context.Context, prize ZenxiangLiyuPrize) (*ZenxiangLiyuPrize, error) {
	return &prize, nil
}

func (r *zenxiangLiyuServiceTestRepository) SavePrizes(_ context.Context, prizes []ZenxiangLiyuPrize) ([]ZenxiangLiyuPrize, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bulkSaveCalls++
	r.prizes = append([]ZenxiangLiyuPrize(nil), prizes...)
	return append([]ZenxiangLiyuPrize(nil), r.prizes...), nil
}

func (r *zenxiangLiyuServiceTestRepository) DeletePrize(context.Context, int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleteCalls++
	return nil
}

func (r *zenxiangLiyuServiceTestRepository) IsUserGranted(context.Context, int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grantCalls++
	return r.granted, nil
}

func (r *zenxiangLiyuServiceTestRepository) GetUserBalance(context.Context, int64) (float64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.balance, nil
}

func (r *zenxiangLiyuServiceTestRepository) CountUserPlaysOnDate(context.Context, int64, time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.countUserPlays, nil
}

func (r *zenxiangLiyuServiceTestRepository) GetUserUsageAmountOnDate(context.Context, int64, time.Time) (float64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.todayUsageAmount, nil
}

func (r *zenxiangLiyuServiceTestRepository) HasUserFreePlayOnDate(context.Context, int64, time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.freePlayUsed, nil
}

func (r *zenxiangLiyuServiceTestRepository) ListUserRecords(context.Context, int64, time.Time, int, int) ([]ZenxiangLiyuRecord, int, error) {
	return nil, 0, nil
}
func (r *zenxiangLiyuServiceTestRepository) GetUserDailySummary(context.Context, int64, time.Time) (*ZenxiangLiyuDailySummary, error) {
	return &ZenxiangLiyuDailySummary{}, nil
}
func (r *zenxiangLiyuServiceTestRepository) ListGrants(context.Context, int, int) ([]ZenxiangLiyuGrant, int, error) {
	return nil, 0, nil
}
func (r *zenxiangLiyuServiceTestRepository) SaveGrant(context.Context, ZenxiangLiyuGrant) (*ZenxiangLiyuGrant, error) {
	return nil, nil
}
func (r *zenxiangLiyuServiceTestRepository) DeleteGrant(context.Context, int64) error { return nil }
func (r *zenxiangLiyuServiceTestRepository) CountGiftedTicketsOnDate(context.Context, int64, time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.giftedTickets, nil
}
func (r *zenxiangLiyuServiceTestRepository) SyncTicketBalance(context.Context, int64, time.Time, ZenxiangLiyuSettings) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ticketBalance, nil
}
func (r *zenxiangLiyuServiceTestRepository) GiftTickets(_ context.Context, gift ZenxiangLiyuTicketGift) (*ZenxiangLiyuTicketGift, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.giftCalls++
	gift.ID = 1
	return &gift, nil
}
func (r *zenxiangLiyuServiceTestRepository) GetOverviewStats(context.Context) (*ZenxiangLiyuOverviewStats, error) {
	return nil, nil
}
func (r *zenxiangLiyuServiceTestRepository) ListUserStats(context.Context, int, int, time.Time) ([]ZenxiangLiyuUserStats, int, error) {
	return nil, 0, nil
}
func (r *zenxiangLiyuServiceTestRepository) ListPrizeStats(context.Context) ([]ZenxiangLiyuPrizeStats, error) {
	return nil, nil
}
func (r *zenxiangLiyuServiceTestRepository) ListPeriodStats(context.Context, string) ([]ZenxiangLiyuPeriodStats, error) {
	return nil, nil
}
func (r *zenxiangLiyuServiceTestRepository) ResetUserDailyPlays(context.Context, int64, time.Time, *int64, string) (int, error) {
	return 0, nil
}

func (r *zenxiangLiyuServiceTestRepository) Play(_ context.Context, command ZenxiangLiyuPlayCommand) (*ZenxiangLiyuPlayResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.playCalls++
	r.lastPlayCommand = command
	return &ZenxiangLiyuPlayResult{Applied: true, RequestID: command.RequestID}, nil
}

func (r *zenxiangLiyuServiceTestRepository) PlayLuckyCoin(context.Context, ZenxiangLiyuLuckyCoinCommand) (*ZenxiangLiyuLuckyCoinResult, error) {
	return &ZenxiangLiyuLuckyCoinResult{}, nil
}

func newZenxiangLiyuServiceTestRepository(globalEnabled, granted bool) *zenxiangLiyuServiceTestRepository {
	return &zenxiangLiyuServiceTestRepository{
		settings:         ZenxiangLiyuSettings{GlobalEnabled: globalEnabled, TicketAmount: 0, MinimumBalance: 0, DailyPlayLimit: 1, TicketUsageThreshold: 5, DailyTicketLimit: 3, UnitSalePrice: 0.1, UnitCostPrice: 0.05, LuckyCoinEnabled: true, LuckyCoinProbability: 50},
		prizes:           []ZenxiangLiyuPrize{{ID: 1, Name: "A", RewardAmount: 1, Probability: 100, Enabled: true}},
		granted:          granted,
		balance:          10,
		todayUsageAmount: 5.01,
		ticketBalance:    1,
	}
}

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

func TestZenxiangLiyuValidatePrizesRejectsNonFiniteRewardAndProbability(t *testing.T) {
	tests := []struct {
		name  string
		prize ZenxiangLiyuPrize
	}{
		{name: "NaN reward", prize: ZenxiangLiyuPrize{ID: 1, Name: "A", RewardAmount: math.NaN(), Probability: 100, Enabled: true}},
		{name: "positive infinity reward", prize: ZenxiangLiyuPrize{ID: 1, Name: "A", RewardAmount: math.Inf(1), Probability: 100, Enabled: true}},
		{name: "negative infinity reward", prize: ZenxiangLiyuPrize{ID: 1, Name: "A", RewardAmount: math.Inf(-1), Probability: 100, Enabled: true}},
		{name: "NaN probability", prize: ZenxiangLiyuPrize{ID: 1, Name: "A", RewardAmount: 1, Probability: math.NaN(), Enabled: true}},
		{name: "positive infinity probability", prize: ZenxiangLiyuPrize{ID: 1, Name: "A", RewardAmount: 1, Probability: math.Inf(1), Enabled: true}},
		{name: "negative infinity probability", prize: ZenxiangLiyuPrize{ID: 1, Name: "A", RewardAmount: 1, Probability: math.Inf(-1), Enabled: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, ValidateZenxiangLiyuPrizes([]ZenxiangLiyuPrize{tt.prize}), ErrZenxiangLiyuInvalidSettings)
		})
	}
}

func TestZenxiangLiyuSavePrizesAtomicallyReplacesValidConfiguration(t *testing.T) {
	repo := newZenxiangLiyuServiceTestRepository(true, false)
	repo.prizes = []ZenxiangLiyuPrize{
		{ID: 1, Name: "A", RewardAmount: 1, Probability: 50, Enabled: true},
		{ID: 2, Name: "B", RewardAmount: 3, Probability: 50, Enabled: true},
	}
	svc := NewZenxiangLiyuService(repo, time.Now, rand.New(rand.NewSource(1)))

	prizes, err := svc.SavePrizes(context.Background(), []ZenxiangLiyuPrizeUpdate{
		{ID: 1, Name: "A", RewardAmount: 1, Probability: 60, Enabled: true},
		{ID: 2, Name: "B", RewardAmount: 3, Probability: 40, Enabled: true},
	})

	require.NoError(t, err)
	require.Equal(t, []ZenxiangLiyuPrize{
		{ID: 1, Name: "A", RewardAmount: 1, Probability: 60, Enabled: true},
		{ID: 2, Name: "B", RewardAmount: 3, Probability: 40, Enabled: true},
	}, prizes)
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, 1, repo.bulkSaveCalls)
	require.Equal(t, prizes, repo.prizes)
}

func TestZenxiangLiyuSavePrizesRejectsInvalidTotalWithoutChangingConfiguration(t *testing.T) {
	repo := newZenxiangLiyuServiceTestRepository(true, false)
	repo.prizes = []ZenxiangLiyuPrize{
		{ID: 1, Name: "A", RewardAmount: 1, Probability: 50, Enabled: true},
		{ID: 2, Name: "B", RewardAmount: 3, Probability: 50, Enabled: true},
	}
	want := append([]ZenxiangLiyuPrize(nil), repo.prizes...)
	svc := NewZenxiangLiyuService(repo, time.Now, rand.New(rand.NewSource(1)))

	_, err := svc.SavePrizes(context.Background(), []ZenxiangLiyuPrizeUpdate{
		{ID: 1, Name: "A", RewardAmount: 1, Probability: 60, Enabled: true},
		{ID: 2, Name: "B", RewardAmount: 3, Probability: 30, Enabled: true},
	})

	require.ErrorIs(t, err, ErrZenxiangLiyuInvalidProbabilityTotal)
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Zero(t, repo.bulkSaveCalls)
	require.Equal(t, want, repo.prizes)
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

func TestZenxiangLiyuPlayDelegatesPolicyToRepositoryTransaction(t *testing.T) {
	repo := newZenxiangLiyuServiceTestRepository(false, false)
	svc := NewZenxiangLiyuService(repo, func() time.Time {
		return time.Date(2026, time.July, 10, 12, 30, 0, 0, time.UTC)
	}, rand.New(rand.NewSource(1)))

	result, err := svc.Play(context.Background(), 42, "req-delegated")

	require.NoError(t, err)
	require.Equal(t, "req-delegated", result.RequestID)
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, 1, repo.playCalls)
	require.Zero(t, repo.grantCalls)
	require.Equal(t, int64(42), repo.lastPlayCommand.UserID)
	require.Equal(t, "req-delegated", repo.lastPlayCommand.RequestID)
	require.Equal(t, time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC), repo.lastPlayCommand.PlayDate)
	require.GreaterOrEqual(t, repo.lastPlayCommand.Roll, 0.0)
	require.Less(t, repo.lastPlayCommand.Roll, 100.0)
}

func TestZenxiangLiyuPlayDateUsesShanghaiDayBoundary(t *testing.T) {
	repo := newZenxiangLiyuServiceTestRepository(false, false)
	svc := NewZenxiangLiyuService(repo, func() time.Time {
		return time.Date(2026, time.July, 10, 16, 30, 0, 0, time.UTC)
	}, rand.New(rand.NewSource(1)))

	_, err := svc.Play(context.Background(), 42, "req-shanghai-day")

	require.NoError(t, err)
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, time.Date(2026, time.July, 11, 0, 0, 0, 0, time.UTC), repo.lastPlayCommand.PlayDate)
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

func TestZenxiangLiyuAuthorization(t *testing.T) {
	tests := []struct {
		name          string
		globalEnabled bool
		granted       bool
		wantStatus    string
		grantCalls    int
	}{
		{name: "global enabled permits ordinary user", globalEnabled: true, granted: false, wantStatus: "", grantCalls: 0},
		{name: "global disabled permits granted user", globalEnabled: false, granted: true, wantStatus: "", grantCalls: 1},
		{name: "global disabled hides ungranted user", globalEnabled: false, granted: false, wantStatus: ErrZenxiangLiyuUnauthorized.Error(), grantCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newZenxiangLiyuServiceTestRepository(tt.globalEnabled, tt.granted)
			svc := NewZenxiangLiyuService(repo, func() time.Time { return time.Unix(0, 0).UTC() }, rand.New(rand.NewSource(1)))

			status, err := svc.GetStatus(context.Background(), 1)
			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, status.Reason)
			require.Equal(t, tt.wantStatus == "", status.Visible)

			result, err := svc.Play(context.Background(), 1, "request-id")
			require.NoError(t, err)
			require.True(t, result.Applied)

			repo.mu.Lock()
			defer repo.mu.Unlock()
			require.Equal(t, tt.grantCalls, repo.grantCalls)
		})
	}
}

func TestZenxiangLiyuStatusRejectsWhenNoTicketAvailable(t *testing.T) {
	repo := newZenxiangLiyuServiceTestRepository(true, false)
	repo.settings = ZenxiangLiyuSettings{GlobalEnabled: true, TicketUsageThreshold: 5, DailyTicketLimit: 3, DailyPlayLimit: 5}
	repo.todayUsageAmount = 4.99
	repo.ticketBalance = 0
	svc := NewZenxiangLiyuService(repo, func() time.Time { return time.Unix(0, 0).UTC() }, rand.New(rand.NewSource(1)))

	status, err := svc.GetStatus(context.Background(), 224)

	require.NoError(t, err)
	require.True(t, status.Visible)
	require.False(t, status.CanPlay)
	require.Equal(t, ErrZenxiangLiyuNoTicket.Error(), status.Reason)
	require.InDelta(t, 10, status.Balance, 0.000001)
	require.Zero(t, status.TodayTicketsAvailable)
	require.InDelta(t, 5, status.NextTicketUsageTarget, 0.000001)
	require.InDelta(t, 0.01, status.NextTicketUsageMissing, 0.000001)
}

func TestZenxiangLiyuStatusAllowsTicketAfterDailyUsageThreshold(t *testing.T) {
	repo := newZenxiangLiyuServiceTestRepository(true, false)
	repo.settings = ZenxiangLiyuSettings{GlobalEnabled: true, TicketUsageThreshold: 5, DailyTicketLimit: 3, DailyPlayLimit: 5}
	repo.balance = 0
	repo.countUserPlays = 0
	repo.todayUsageAmount = 5.01
	repo.ticketBalance = 1
	svc := NewZenxiangLiyuService(repo, func() time.Time { return time.Unix(0, 0).UTC() }, rand.New(rand.NewSource(1)))

	status, err := svc.GetStatus(context.Background(), 224)

	require.NoError(t, err)
	require.True(t, status.CanPlay)
	require.Zero(t, status.EffectiveTicketAmount)
	require.InDelta(t, 5.01, status.TodayUsageAmount, 0.000001)
	require.Equal(t, 1, status.TodayTicketsEarned)
	require.Equal(t, 1, status.TodayTicketsAvailable)
	require.Equal(t, 1, status.TicketsAvailable)
	require.Equal(t, 5, status.TicketCapacity)
	require.Equal(t, 2, status.TicketRetentionDays)
	require.InDelta(t, 10, status.NextTicketUsageTarget, 0.000001)
	require.InDelta(t, 4.99, status.NextTicketUsageMissing, 0.000001)
}

func TestZenxiangLiyuStatusIncludesGiftedTickets(t *testing.T) {
	repo := newZenxiangLiyuServiceTestRepository(true, false)
	repo.settings = ZenxiangLiyuSettings{GlobalEnabled: true, TicketUsageThreshold: 5, DailyTicketLimit: 3, DailyPlayLimit: 5}
	repo.todayUsageAmount = 0
	repo.giftedTickets = 2
	repo.ticketBalance = 2
	svc := NewZenxiangLiyuService(repo, func() time.Time { return time.Unix(0, 0).UTC() }, rand.New(rand.NewSource(1)))

	status, err := svc.GetStatus(context.Background(), 224)

	require.NoError(t, err)
	require.True(t, status.CanPlay)
	require.Equal(t, 0, status.TodayTicketsFromUsage)
	require.Equal(t, 2, status.TodayTicketsGranted)
	require.Equal(t, 2, status.TodayTicketsEarned)
	require.Equal(t, 2, status.TodayTicketsAvailable)
	require.Equal(t, 2, status.TicketsAvailable)
	require.Equal(t, 5, status.TicketCapacity)
	require.Equal(t, 2, status.TicketRetentionDays)
	require.InDelta(t, 5, status.NextTicketUsageTarget, 0.000001)
}

func TestZenxiangLiyuGiftTicketsUsesTodayAndValidatesCount(t *testing.T) {
	repo := newZenxiangLiyuServiceTestRepository(true, false)
	playDate := time.Date(2026, time.July, 11, 0, 0, 0, 0, time.UTC)
	svc := NewZenxiangLiyuService(repo, func() time.Time { return playDate.Add(2 * time.Hour) }, rand.New(rand.NewSource(1)))

	grantedBy := int64(9)
	gift, err := svc.GiftTickets(context.Background(), ZenxiangLiyuTicketGiftRequest{RequestID: "gift-1", UserID: 224, TicketCount: 3, GrantedBy: &grantedBy, Notes: " 客服补偿 "})

	require.NoError(t, err)
	require.Equal(t, "gift-1", gift.RequestID)
	require.Equal(t, int64(224), gift.UserID)
	require.Equal(t, 3, gift.TicketCount)
	require.Equal(t, playDate, gift.PlayDate)
	require.Equal(t, "客服补偿", gift.Notes)
	repo.mu.Lock()
	require.Equal(t, 1, repo.giftCalls)
	repo.mu.Unlock()

	_, err = svc.GiftTickets(context.Background(), ZenxiangLiyuTicketGiftRequest{UserID: 224, TicketCount: 0})
	require.ErrorIs(t, err, ErrZenxiangLiyuInvalidSettings)
	_, err = svc.GiftTickets(context.Background(), ZenxiangLiyuTicketGiftRequest{UserID: 224, TicketCount: 1})
	require.ErrorIs(t, err, ErrZenxiangLiyuInvalidSettings)
}

func TestZenxiangLiyuDeletePrizeRejectsInvalidRemainingConfiguration(t *testing.T) {
	repo := newZenxiangLiyuServiceTestRepository(true, false)
	svc := NewZenxiangLiyuService(repo, time.Now, rand.New(rand.NewSource(1)))

	err := svc.DeletePrize(context.Background(), 1)
	require.ErrorIs(t, err, ErrZenxiangLiyuInvalidProbabilityTotal)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Zero(t, repo.deleteCalls)
}

func TestZenxiangLiyuSimulateSupportsConcurrentRandomUse(t *testing.T) {
	svc := NewZenxiangLiyuService(nil, time.Now, rand.New(rand.NewSource(1)))
	req := ZenxiangLiyuSimulationRequest{
		UserCount: 1, PlaysPerUser: 10, InitialBalance: 100, TicketAmount: 1, DailyPlayLimit: 10,
		Prizes: []ZenxiangLiyuPrize{{ID: 1, Name: "A", RewardAmount: 1, Probability: 100, Enabled: true}},
	}

	const workers = 32
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Simulate(context.Background(), req)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
