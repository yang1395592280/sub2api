package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type sizeBetQueryRepoStub struct {
	currentRound *SizeBetRound
	latestRounds []SizeBetRound
	currentBet   *SizeBet

	roundsDue []SizeBetRound

	refundBets []SizeBet

	historyItems       []SizeBetUserHistoryItem
	historyPagination  *pagination.PaginationResult
	historyErr         error
	historyUserID      int64
	historyPaginationQ pagination.PaginationParams

	applySettlementCalls int
	createRoundCalls     int
	refreshCalls         int
	refundCalls          int

	lastRefundRoundID int64
	lastRefundedAt    time.Time
	lastCreatedRound  *SizeBetRound

	getRoundByIDErr   error
	getRoundByTimeErr error
	createRoundErr    error
	refundErr         error
}

func (s *sizeBetQueryRepoStub) GetRoundByID(context.Context, int64) (*SizeBetRound, error) {
	if s.getRoundByIDErr != nil {
		return nil, s.getRoundByIDErr
	}
	return s.currentRound, nil
}

func (s *sizeBetQueryRepoStub) GetRoundByTime(context.Context, time.Time) (*SizeBetRound, error) {
	if s.getRoundByTimeErr != nil {
		return nil, s.getRoundByTimeErr
	}
	return s.currentRound, nil
}

func (s *sizeBetQueryRepoStub) CreateRound(_ context.Context, round *SizeBetRound) (*SizeBetRound, error) {
	s.createRoundCalls++
	if s.createRoundErr != nil {
		return nil, s.createRoundErr
	}
	s.lastCreatedRound = round
	s.currentRound = round
	return round, nil
}

func (s *sizeBetQueryRepoStub) CreateBetAndDebit(context.Context, *SizeBet, *SizeBetLedgerEntry) error {
	panic("unexpected CreateBetAndDebit call")
}

func (s *sizeBetQueryRepoStub) ApplySettlement(_ context.Context, _ SettleRoundInput) ([]SizeBet, error) {
	s.applySettlementCalls++
	if s.currentRound != nil {
		s.currentRound.Status = SizeBetRoundStatusSettled
	}
	return nil, nil
}

func (s *sizeBetQueryRepoStub) RefreshLeaderboardSnapshots(context.Context, int64) error {
	s.refreshCalls++
	return nil
}

func (s *sizeBetQueryRepoStub) GetBetByRoundAndUser(context.Context, int64, int64) (*SizeBet, error) {
	return s.currentBet, nil
}

func (s *sizeBetQueryRepoStub) ListRecentRounds(context.Context, int) ([]SizeBetRound, error) {
	return append([]SizeBetRound(nil), s.latestRounds...), nil
}

func (s *sizeBetQueryRepoStub) ListUserHistory(_ context.Context, userID int64, params pagination.PaginationParams) ([]SizeBetUserHistoryItem, *pagination.PaginationResult, error) {
	s.historyUserID = userID
	s.historyPaginationQ = params
	if s.historyErr != nil {
		return nil, nil, s.historyErr
	}
	return append([]SizeBetUserHistoryItem(nil), s.historyItems...), s.historyPagination, nil
}

func (s *sizeBetQueryRepoStub) ListLeaderboard(context.Context, string, string, int) ([]SizeBetLeaderboardEntry, time.Time, error) {
	return nil, time.Time{}, nil
}

func (s *sizeBetQueryRepoStub) ListAdminRounds(context.Context, pagination.PaginationParams) ([]SizeBetRound, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *sizeBetQueryRepoStub) ListAdminBets(context.Context, pagination.PaginationParams, SizeBetAdminBetFilter) ([]SizeBetAdminBet, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *sizeBetQueryRepoStub) ListAdminLedger(context.Context, pagination.PaginationParams, SizeBetAdminLedgerFilter) ([]SizeBetLedgerEntry, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *sizeBetQueryRepoStub) RefundRound(_ context.Context, roundID int64, refundedAt time.Time) ([]SizeBet, error) {
	s.refundCalls++
	s.lastRefundRoundID = roundID
	s.lastRefundedAt = refundedAt
	if s.refundErr != nil {
		return nil, s.refundErr
	}
	return append([]SizeBet(nil), s.refundBets...), nil
}

func (s *sizeBetQueryRepoStub) ListRoundsDueForSettlement(context.Context, time.Time, int) ([]SizeBetRound, error) {
	return append([]SizeBetRound(nil), s.roundsDue...), nil
}

type sizeBetRuntimeSettingRepoStub struct {
	values map[string]string
}

func (s *sizeBetRuntimeSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *sizeBetRuntimeSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *sizeBetRuntimeSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *sizeBetRuntimeSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func (s *sizeBetRuntimeSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = s.values[key]
	}
	return result, nil
}

func (s *sizeBetRuntimeSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *sizeBetRuntimeSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func TestSizeBetServiceGetCurrentRoundViewReturnsCurrentPhaseAndPreviousRound(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 10, 0, time.UTC)
	repo := &sizeBetQueryRepoStub{
		currentRound: &SizeBetRound{
			ID:             11,
			RoundNo:        1001,
			Status:         SizeBetRoundStatusOpen,
			StartsAt:       now.Add(-10 * time.Second),
			BetClosesAt:    now.Add(20 * time.Second),
			SettlesAt:      now.Add(50 * time.Second),
			ProbSmall:      45,
			ProbMid:        10,
			ProbBig:        45,
			OddsSmall:      2,
			OddsMid:        10,
			OddsBig:        2,
			AllowedStakes:  []int{2, 5, 10},
			ServerSeedHash: "hash-1",
		},
		currentBet: &SizeBet{
			ID:          22,
			RoundID:     11,
			UserID:      9,
			Direction:   SizeBetDirectionBig,
			StakeAmount: 10,
			Status:      SizeBetStatusPlaced,
		},
		latestRounds: []SizeBetRound{
			{
				ID:              10,
				RoundNo:         1000,
				Status:          SizeBetRoundStatusSettled,
				ResultNumber:    sizeBetIntPtr(6),
				ResultDirection: SizeBetDirectionMid,
				ServerSeedHash:  "hash-0",
				ServerSeed:      "seed-0",
			},
		},
	}
	svc := NewSizeBetService(repo, &sizeBetRuntimeSettingRepoStub{values: map[string]string{}}, nil, nil)

	view, err := svc.GetCurrentRoundView(context.Background(), 9, now)

	require.NoError(t, err)
	require.True(t, view.Enabled)
	require.Equal(t, SizeBetPhaseBetting, view.Phase)
	require.NotNil(t, view.Round)
	require.Equal(t, int64(11), view.Round.ID)
	require.Equal(t, int64(22), view.MyBet.ID)
	require.NotNil(t, view.PreviousRound)
	require.Equal(t, int64(10), view.PreviousRound.ID)
	require.Equal(t, 20, view.Round.BetCountdownSeconds)
}

func TestSizeBetServiceGetCurrentRoundViewDisabledReturnsMaintenanceWithoutMutation(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 10, 0, time.UTC)
	repo := &sizeBetQueryRepoStub{
		latestRounds: []SizeBetRound{
			{
				ID:              10,
				RoundNo:         1000,
				Status:          SizeBetRoundStatusSettled,
				ResultNumber:    sizeBetIntPtr(6),
				ResultDirection: SizeBetDirectionMid,
				ServerSeedHash:  "hash-0",
				ServerSeed:      "seed-0",
			},
		},
	}
	svc := NewSizeBetService(repo, &sizeBetRuntimeSettingRepoStub{values: map[string]string{
		SettingKeySizeBetEnabled: "false",
	}}, nil, nil)

	view, err := svc.GetCurrentRoundView(context.Background(), 9, now)

	require.NoError(t, err)
	require.False(t, view.Enabled)
	require.Equal(t, SizeBetPhaseMaintenance, view.Phase)
	require.Nil(t, view.Round)
	require.NotNil(t, view.PreviousRound)
	require.Zero(t, repo.createRoundCalls)
}

func TestSizeBetServiceGetHistory(t *testing.T) {
	placedAt := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	settledAt := time.Date(2026, 4, 23, 12, 1, 0, 0, time.UTC)
	balanceAfter := 100.0
	repo := &sizeBetQueryRepoStub{
		historyItems: []SizeBetUserHistoryItem{
			{
				BetID:           7,
				RoundID:         11,
				RoundNo:         1002,
				Direction:       SizeBetDirectionBig,
				ResultNumber:    sizeBetIntPtr(9),
				ResultDirection: SizeBetDirectionBig,
				StakeAmount:     10,
				PayoutAmount:    20,
				NetResultAmount: 10,
				Status:          SizeBetStatusWon,
				BalanceAfter:    &balanceAfter,
				PlacedAt:        placedAt,
				SettledAt:       &settledAt,
			},
		},
		historyPagination: &pagination.PaginationResult{
			Total:    1,
			Page:     1,
			PageSize: 20,
			Pages:    1,
		},
	}
	svc := NewSizeBetService(repo, &sizeBetRuntimeSettingRepoStub{values: map[string]string{}}, nil, nil)

	items, pageResult, err := svc.GetHistory(context.Background(), 9, pagination.PaginationParams{})

	require.NoError(t, err)
	require.Equal(t, int64(9), repo.historyUserID)
	require.Equal(t, 1, repo.historyPaginationQ.Page)
	require.Equal(t, 20, repo.historyPaginationQ.PageSize)
	require.Equal(t, pagination.SortOrderDesc, repo.historyPaginationQ.SortOrder)
	require.Len(t, items, 1)
	require.Equal(t, int64(7), items[0].BetID)
	require.Equal(t, SizeBetDirectionBig, items[0].ResultDirection)
	require.NotNil(t, items[0].ResultNumber)
	require.Equal(t, 9, *items[0].ResultNumber)
	require.NotNil(t, items[0].BalanceAfter)
	require.Equal(t, balanceAfter, *items[0].BalanceAfter)
	require.Equal(t, repo.historyPagination, pageResult)
}

func TestSizeBetServiceRefundRoundRejectsSettledResultRound(t *testing.T) {
	repo := &sizeBetQueryRepoStub{
		currentRound: &SizeBetRound{
			ID:              11,
			Status:          SizeBetRoundStatusSettled,
			ResultNumber:    sizeBetIntPtr(6),
			ResultDirection: SizeBetDirectionMid,
		},
	}
	svc := NewSizeBetService(repo, &sizeBetRuntimeSettingRepoStub{values: map[string]string{}}, nil, nil)

	result, err := svc.RefundRound(context.Background(), 11, time.Now())

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSizeBetRoundAlreadySettled)
	require.Zero(t, repo.refundCalls)
}

func TestSizeBetRuntimeServiceRunOnceEnsuresCurrentRoundAndSettlesDueRounds(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 1, 0, 0, time.UTC)
	repo := &sizeBetQueryRepoStub{
		roundsDue: []SizeBetRound{
			{
				ID:             10,
				RoundNo:        1000,
				Status:         SizeBetRoundStatusOpen,
				StartsAt:       now.Add(-1 * time.Minute),
				BetClosesAt:    now.Add(-10 * time.Second),
				SettlesAt:      now.Add(-1 * time.Second),
				ProbSmall:      45,
				ProbMid:        10,
				ProbBig:        45,
				OddsSmall:      2,
				OddsMid:        10,
				OddsBig:        2,
				AllowedStakes:  []int{2, 5, 10},
				ServerSeedHash: "hash-10",
				ServerSeed:     "seed-10",
			},
		},
	}
	service := NewSizeBetService(repo, &sizeBetRuntimeSettingRepoStub{values: map[string]string{}}, nil, nil)
	service.now = func() time.Time { return now }
	runtime := NewSizeBetRuntimeService(service, time.Second)

	runtime.runOnce()

	require.Equal(t, 1, repo.applySettlementCalls)
	require.Equal(t, 1, repo.refreshCalls)
	require.Equal(t, 1, repo.createRoundCalls)
	require.NotNil(t, repo.lastCreatedRound)
	require.Equal(t, SizeBetGameKey, repo.lastCreatedRound.GameKey)
}

func TestSizeBetRuntimeServiceRunOnceDisabledSkipsMutation(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 1, 0, 0, time.UTC)
	repo := &sizeBetQueryRepoStub{
		roundsDue: []SizeBetRound{
			{
				ID:             10,
				RoundNo:        1000,
				Status:         SizeBetRoundStatusOpen,
				StartsAt:       now.Add(-1 * time.Minute),
				BetClosesAt:    now.Add(-10 * time.Second),
				SettlesAt:      now.Add(-1 * time.Second),
				ServerSeedHash: "hash-10",
				ServerSeed:     "seed-10",
			},
		},
	}
	service := NewSizeBetService(repo, &sizeBetRuntimeSettingRepoStub{values: map[string]string{
		SettingKeySizeBetEnabled: "false",
	}}, nil, nil)
	service.now = func() time.Time { return now }
	runtime := NewSizeBetRuntimeService(service, time.Second)

	runtime.runOnce()

	require.Zero(t, repo.applySettlementCalls)
	require.Zero(t, repo.createRoundCalls)
	require.Zero(t, repo.refreshCalls)
}

func TestSizeBetRuntimeServiceRunOnceToleratesEnsureRoundFailure(t *testing.T) {
	repo := &sizeBetQueryRepoStub{getRoundByTimeErr: errors.New("boom")}
	service := NewSizeBetService(repo, &sizeBetRuntimeSettingRepoStub{values: map[string]string{}}, nil, nil)
	runtime := NewSizeBetRuntimeService(service, time.Second)

	require.NotPanics(t, func() {
		runtime.runOnce()
	})
}

func TestSizeBetRuntimeServiceStopStopsBackgroundWorker(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 1, 0, 0, time.UTC)
	repo := &sizeBetQueryRepoStub{
		roundsDue: []SizeBetRound{
			{
				ID:             10,
				RoundNo:        1000,
				Status:         SizeBetRoundStatusOpen,
				StartsAt:       now.Add(-1 * time.Minute),
				BetClosesAt:    now.Add(-10 * time.Second),
				SettlesAt:      now.Add(-1 * time.Second),
				ProbSmall:      45,
				ProbMid:        10,
				ProbBig:        45,
				OddsSmall:      2,
				OddsMid:        10,
				OddsBig:        2,
				AllowedStakes:  []int{2, 5, 10},
				ServerSeedHash: "hash-10",
				ServerSeed:     "seed-10",
			},
		},
	}
	service := NewSizeBetService(repo, &sizeBetRuntimeSettingRepoStub{values: map[string]string{}}, nil, nil)
	service.now = func() time.Time { return now }
	runtime := NewSizeBetRuntimeService(service, 10*time.Millisecond)

	runtime.Start()
	time.Sleep(35 * time.Millisecond)
	runtime.Stop()

	callsAfterStop := repo.applySettlementCalls
	time.Sleep(30 * time.Millisecond)

	require.Equal(t, callsAfterStop, repo.applySettlementCalls)
}
