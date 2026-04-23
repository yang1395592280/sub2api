package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sizeBetRepoStub struct {
	currentRound   *SizeBetRound
	createdRound   *SizeBetRound
	bets           []SizeBet
	ledgerEntries  []SizeBetLedgerEntry
	userBalance    float64
	settlementRuns []SettleRoundInput
	err            error
}

func newSizeBetRepoStub() *sizeBetRepoStub {
	return &sizeBetRepoStub{}
}

func (s *sizeBetRepoStub) GetRoundByID(context.Context, int64) (*SizeBetRound, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.currentRound, nil
}

func (s *sizeBetRepoStub) GetRoundByTime(context.Context, time.Time) (*SizeBetRound, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.currentRound, nil
}

func (s *sizeBetRepoStub) CreateRound(_ context.Context, round *SizeBetRound) (*SizeBetRound, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.createdRound = round
	s.currentRound = round
	return round, nil
}

func (s *sizeBetRepoStub) CreateBetAndDebit(_ context.Context, bet *SizeBet, entry *SizeBetLedgerEntry) error {
	if s.err != nil {
		return s.err
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
	if s.err != nil {
		return nil, s.err
	}
	return append([]SizeBet(nil), s.bets...), nil
}

func (s *sizeBetRepoStub) ApplySettlement(_ context.Context, input SettleRoundInput, bets []SizeBet) error {
	if s.err != nil {
		return s.err
	}
	s.settlementRuns = append(s.settlementRuns, input)
	for _, bet := range bets {
		if bet.Direction != input.ResultDirection {
			continue
		}
		entry := NewBetPayoutLedger(&SizeBetRound{ID: input.RoundID}, &bet, input.OddsFor(bet.Direction))
		s.ledgerEntries = append(s.ledgerEntries, *entry)
	}
	return nil
}

func (s *sizeBetRepoStub) RefreshLeaderboardSnapshots(context.Context, int64) error {
	return s.err
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
}
