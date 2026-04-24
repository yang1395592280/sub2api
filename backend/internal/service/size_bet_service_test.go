package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type sizeBetRepoStub struct {
	currentRound            *SizeBetRound
	createdRound            *SizeBetRound
	bets                    []SizeBet
	ledgerEntries           []SizeBetLedgerEntry
	userBalance             float64
	settlementRuns          []SettleRoundInput
	getRoundByIDCalls       int
	getRoundByTimeCalls     int
	getRoundByIDErr         error
	getRoundByTimeErr       error
	createRoundErr          error
	createBetErr            error
	loadRoundBetsErr        error
	applySettlementErr      error
	refreshLeaderboardErr   error
	createRoundCalls        int
	refreshLeaderboardCalls int
	loadRoundBetsCalls      int
	applySettlementCalls    int
}

func newSizeBetRepoStub() *sizeBetRepoStub {
	return &sizeBetRepoStub{}
}

func (s *sizeBetRepoStub) GetRoundByID(context.Context, int64) (*SizeBetRound, error) {
	s.getRoundByIDCalls++
	if s.getRoundByIDErr != nil {
		return nil, s.getRoundByIDErr
	}
	return s.currentRound, nil
}

func (s *sizeBetRepoStub) GetRoundByTime(context.Context, time.Time) (*SizeBetRound, error) {
	s.getRoundByTimeCalls++
	if s.getRoundByTimeErr != nil {
		return nil, s.getRoundByTimeErr
	}
	return s.currentRound, nil
}

func (s *sizeBetRepoStub) CreateRound(_ context.Context, round *SizeBetRound) (*SizeBetRound, error) {
	s.createRoundCalls++
	if s.createRoundErr != nil {
		return nil, s.createRoundErr
	}
	s.createdRound = round
	s.currentRound = round
	return round, nil
}

func (s *sizeBetRepoStub) CreateBetAndDebit(_ context.Context, bet *SizeBet, entry *SizeBetLedgerEntry) error {
	if s.createBetErr != nil {
		return s.createBetErr
	}
	s.userBalance -= bet.StakeAmount
	if entry != nil {
		entry.BalanceBefore = s.userBalance + bet.StakeAmount
		entry.BalanceAfter = s.userBalance
		s.ledgerEntries = append(s.ledgerEntries, *entry)
	}
	return nil
}

func (s *sizeBetRepoStub) LoadRoundBetsForSettlement(context.Context, int64) ([]SizeBet, error) {
	s.loadRoundBetsCalls++
	if s.loadRoundBetsErr != nil {
		return nil, s.loadRoundBetsErr
	}
	return append([]SizeBet(nil), s.bets...), nil
}

func (s *sizeBetRepoStub) ApplySettlement(_ context.Context, input SettleRoundInput) ([]SizeBet, error) {
	s.applySettlementCalls++
	if s.applySettlementErr != nil {
		return nil, s.applySettlementErr
	}
	bets := append([]SizeBet(nil), s.bets...)
	s.settlementRuns = append(s.settlementRuns, input)
	for _, bet := range bets {
		if bet.Direction != input.ResultDirection {
			continue
		}
		entry := NewBetPayoutLedger(&SizeBetRound{ID: input.RoundID}, &bet, input.OddsFor(bet.Direction))
		s.ledgerEntries = append(s.ledgerEntries, *entry)
	}
	if s.currentRound != nil {
		s.currentRound.Status = SizeBetRoundStatusSettled
		s.currentRound.ResultNumber = sizeBetIntPtr(input.ResultNumber)
		s.currentRound.ResultDirection = input.ResultDirection
		if input.ServerSeed != "" {
			s.currentRound.ServerSeed = input.ServerSeed
		}
	}
	return bets, nil
}

func (s *sizeBetRepoStub) RefreshLeaderboardSnapshots(context.Context, int64) error {
	s.refreshLeaderboardCalls++
	return s.refreshLeaderboardErr
}

func (s *sizeBetRepoStub) GetBetByRoundAndUser(context.Context, int64, int64) (*SizeBet, error) {
	return nil, nil
}

func (s *sizeBetRepoStub) ListRecentRounds(context.Context, int) ([]SizeBetRound, error) {
	return nil, nil
}

func (s *sizeBetRepoStub) ListUserHistory(context.Context, int64, pagination.PaginationParams) ([]SizeBetUserHistoryItem, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *sizeBetRepoStub) ListLeaderboard(context.Context, string, string, int) ([]SizeBetLeaderboardEntry, time.Time, error) {
	return nil, time.Time{}, nil
}

func (s *sizeBetRepoStub) ListAdminRounds(context.Context, pagination.PaginationParams) ([]SizeBetRound, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *sizeBetRepoStub) ListAdminBets(context.Context, pagination.PaginationParams, SizeBetAdminBetFilter) ([]SizeBetAdminBet, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *sizeBetRepoStub) ListAdminLedger(context.Context, pagination.PaginationParams, SizeBetAdminLedgerFilter) ([]SizeBetLedgerEntry, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *sizeBetRepoStub) RefundRound(context.Context, int64, time.Time) ([]SizeBet, error) {
	return nil, nil
}

func (s *sizeBetRepoStub) ListRoundsDueForSettlement(context.Context, time.Time, int) ([]SizeBetRound, error) {
	return nil, nil
}

func (s *sizeBetRepoStub) GetStatsOverview(context.Context, string) (*SizeBetStatsOverview, error) {
	return nil, nil
}

func (s *sizeBetRepoStub) ListStatsUsers(context.Context, string, pagination.PaginationParams) ([]SizeBetStatsUserItem, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func mustOpenRound() *SizeBetRound {
	now := time.Now()
	return &SizeBetRound{
		ID:            11,
		GameKey:       SizeBetGameKey,
		RoundNo:       1001,
		Status:        SizeBetRoundStatusOpen,
		StartsAt:      now.Add(-10 * time.Second),
		BetClosesAt:   now.Add(10 * time.Second),
		SettlesAt:     now.Add(50 * time.Second),
		AllowedStakes: []int{2, 5, 10, 20},
		OddsSmall:     2,
		OddsMid:       10,
		OddsBig:       2,
	}
}

func TestSizeBetServicePlaceBetDeductsBalanceAndCreatesLedger(t *testing.T) {
	repo := newSizeBetRepoStub()
	svc := NewSizeBetService(repo, &sizeBetSettingRepoStub{values: map[string]string{}}, nil, nil)

	round := mustOpenRound()
	repo.currentRound = round
	repo.userBalance = 20

	bet, err := svc.PlaceBet(context.Background(), PlaceSizeBetRequest{
		UserID:      7,
		RoundID:     round.ID,
		Direction:   SizeBetDirectionSmall,
		StakeAmount: 5,
	})

	require.NoError(t, err)
	require.Equal(t, 5.0, bet.StakeAmount)
	require.Len(t, repo.ledgerEntries, 1)
	require.Equal(t, "bet_debit", repo.ledgerEntries[0].EntryType)
	require.InDelta(t, 15, repo.userBalance, 0.001)
}

func TestSizeBetServiceSettleRoundCreditsWinningBets(t *testing.T) {
	repo := newSizeBetRepoStub()
	svc := NewSizeBetService(repo, &sizeBetSettingRepoStub{values: map[string]string{}}, nil, nil)

	repo.currentRound = mustOpenRound()
	repo.bets = []SizeBet{
		{ID: 1, UserID: 7, Direction: SizeBetDirectionMid, StakeAmount: 2, Status: SizeBetStatusPlaced},
	}

	err := svc.SettleRound(context.Background(), SettleRoundInput{
		RoundID:         11,
		ResultNumber:    6,
		ResultDirection: SizeBetDirectionMid,
		OddsMid:         10,
	})

	require.NoError(t, err)
	require.Len(t, repo.ledgerEntries, 1)
	require.Equal(t, "bet_payout", repo.ledgerEntries[0].EntryType)
	require.InDelta(t, 20, repo.ledgerEntries[0].DeltaAmount, 0.001)
	require.Equal(t, 1, repo.refreshLeaderboardCalls)
}

func TestSizeBetServiceEnsureCurrentRoundReturnsLookupError(t *testing.T) {
	repo := newSizeBetRepoStub()
	repo.getRoundByTimeErr = errors.New("lookup failed")
	svc := NewSizeBetService(repo, &sizeBetSettingRepoStub{values: map[string]string{}}, nil, nil)

	round, err := svc.EnsureCurrentRound(context.Background(), time.Now())

	require.Nil(t, round)
	require.ErrorIs(t, err, repo.getRoundByTimeErr)
	require.Zero(t, repo.createRoundCalls)
	require.Nil(t, repo.createdRound)
}

func TestSizeBetServiceEnsureCurrentRoundDisabledDoesNotCreateRound(t *testing.T) {
	repo := newSizeBetRepoStub()
	svc := NewSizeBetService(repo, &sizeBetSettingRepoStub{
		values: map[string]string{
			SettingKeySizeBetEnabled: "false",
		},
	}, nil, nil)

	round, err := svc.EnsureCurrentRound(context.Background(), time.Now())

	require.NoError(t, err)
	require.Nil(t, round)
	require.Equal(t, 0, repo.createRoundCalls)
}

func TestSizeBetServiceEnsureCurrentRoundPreparationWindowDoesNotCreateRound(t *testing.T) {
	repo := newSizeBetRepoStub()
	svc := NewSizeBetService(repo, &sizeBetSettingRepoStub{values: map[string]string{}}, nil, nil)

	round, err := svc.EnsureCurrentRound(context.Background(), time.Unix(65, 0).UTC())

	require.NoError(t, err)
	require.Nil(t, round)
	require.Equal(t, 0, repo.createRoundCalls)
}

func TestSizeBetServiceSettleRoundRejectsInvalidResultPayload(t *testing.T) {
	testCases := []struct {
		name   string
		number int
		dir    SizeBetDirection
	}{
		{name: "small range cannot settle as mid", number: 1, dir: SizeBetDirectionMid},
		{name: "mid cannot settle as big", number: 6, dir: SizeBetDirectionBig},
		{name: "big range cannot settle as small", number: 11, dir: SizeBetDirectionSmall},
		{name: "out of range number rejected", number: 12, dir: SizeBetDirectionBig},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newSizeBetRepoStub()
			repo.currentRound = mustOpenRound()
			svc := NewSizeBetService(repo, &sizeBetSettingRepoStub{values: map[string]string{}}, nil, nil)

			err := svc.SettleRound(context.Background(), SettleRoundInput{
				RoundID:         repo.currentRound.ID,
				ResultNumber:    tc.number,
				ResultDirection: tc.dir,
			})

			require.ErrorIs(t, err, ErrSizeBetInvalidResult)
			require.Zero(t, repo.getRoundByIDCalls)
			require.Zero(t, repo.loadRoundBetsCalls)
			require.Zero(t, repo.applySettlementCalls)
			require.Zero(t, repo.refreshLeaderboardCalls)
			require.Empty(t, repo.settlementRuns)
		})
	}
}

func TestSizeBetServiceSettleRoundRejectsAlreadySettledRound(t *testing.T) {
	repo := newSizeBetRepoStub()
	round := mustOpenRound()
	round.Status = SizeBetRoundStatusSettled
	repo.currentRound = round
	svc := NewSizeBetService(repo, &sizeBetSettingRepoStub{values: map[string]string{}}, nil, nil)

	err := svc.SettleRound(context.Background(), SettleRoundInput{
		RoundID:         round.ID,
		ResultNumber:    6,
		ResultDirection: SizeBetDirectionMid,
	})

	require.ErrorIs(t, err, ErrSizeBetRoundAlreadySettled)
	require.Zero(t, repo.loadRoundBetsCalls)
	require.Zero(t, repo.applySettlementCalls)
	require.Zero(t, repo.refreshLeaderboardCalls)
}

func TestSizeBetServiceSettleRoundAllowsRefreshRetryForMatchingSettledOutcome(t *testing.T) {
	repo := newSizeBetRepoStub()
	repo.currentRound = mustOpenRound()
	repo.bets = []SizeBet{
		{ID: 1, UserID: 7, Direction: SizeBetDirectionMid, StakeAmount: 2, Status: SizeBetStatusPlaced},
	}
	repo.refreshLeaderboardErr = errors.New("refresh failed")
	svc := NewSizeBetService(repo, &sizeBetSettingRepoStub{values: map[string]string{}}, nil, nil)

	input := SettleRoundInput{
		RoundID:         repo.currentRound.ID,
		ResultNumber:    6,
		ResultDirection: SizeBetDirectionMid,
		OddsMid:         10,
		ServerSeed:      "server-seed-1",
	}

	err := svc.SettleRound(context.Background(), input)
	require.ErrorIs(t, err, repo.refreshLeaderboardErr)
	require.Equal(t, 1, repo.applySettlementCalls)
	require.Equal(t, 1, repo.refreshLeaderboardCalls)
	require.Len(t, repo.ledgerEntries, 1)

	repo.refreshLeaderboardErr = nil
	err = svc.SettleRound(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 1, repo.applySettlementCalls)
	require.Equal(t, 2, repo.refreshLeaderboardCalls)
	require.Len(t, repo.ledgerEntries, 1)
}

func TestSizeBetServiceSettleRoundRejectsRefreshRetryWithMismatchedSettledOutcome(t *testing.T) {
	repo := newSizeBetRepoStub()
	round := mustOpenRound()
	round.Status = SizeBetRoundStatusSettled
	round.ResultNumber = sizeBetIntPtr(6)
	round.ResultDirection = SizeBetDirectionMid
	repo.currentRound = round
	svc := NewSizeBetService(repo, &sizeBetSettingRepoStub{values: map[string]string{}}, nil, nil)

	err := svc.SettleRound(context.Background(), SettleRoundInput{
		RoundID:         round.ID,
		ResultNumber:    7,
		ResultDirection: SizeBetDirectionBig,
	})

	require.ErrorIs(t, err, ErrSizeBetSettlementConflict)
	require.Zero(t, repo.applySettlementCalls)
	require.Zero(t, repo.refreshLeaderboardCalls)
}

func sizeBetIntPtr(v int) *int {
	return &v
}
