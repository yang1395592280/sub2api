//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSizeBetRepositoryCreateBetAndDebit_DebitsBalanceAndCreatesLedger(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewSizeBetRepository(client, integrationDB).(*sizeBetRepository)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("size-bet-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      20,
	})
	round := mustInsertSizeBetRound(t, ctx, sizeBetRoundInsertInput{status: service.SizeBetRoundStatusOpen})

	bet := &service.SizeBet{
		RoundID:        round.ID,
		UserID:         user.ID,
		Direction:      service.SizeBetDirectionSmall,
		StakeAmount:    5,
		Status:         service.SizeBetStatusPlaced,
		IdempotencyKey: "bet-" + uuid.NewString(),
	}
	entry := service.NewBetDebitLedger(round, bet)

	err := repo.CreateBetAndDebit(ctx, bet, entry)
	require.NoError(t, err)
	require.NotZero(t, bet.ID)
	require.NotZero(t, entry.ID)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1`, user.ID).Scan(&balance))
	require.InDelta(t, 15, balance, 0.000001)

	var betCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_bets WHERE id = $1 AND status = $2`, bet.ID, service.SizeBetStatusPlaced).Scan(&betCount))
	require.Equal(t, 1, betCount)

	var entryType string
	var deltaAmount float64
	var balanceBefore float64
	var balanceAfter float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT entry_type, delta_amount, balance_before, balance_after
		FROM game_wallet_ledger
		WHERE id = $1
	`, entry.ID).Scan(&entryType, &deltaAmount, &balanceBefore, &balanceAfter))
	require.Equal(t, "bet_debit", entryType)
	require.InDelta(t, -5, deltaAmount, 0.000001)
	require.InDelta(t, 20, balanceBefore, 0.000001)
	require.InDelta(t, 15, balanceAfter, 0.000001)
}

func TestSizeBetRepositoryCreateBetAndDebit_MapsDuplicateConflictAndRollsBackDebit(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewSizeBetRepository(client, integrationDB).(*sizeBetRepository)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("size-bet-dup-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      20,
	})
	round := mustInsertSizeBetRound(t, ctx, sizeBetRoundInsertInput{status: service.SizeBetRoundStatusOpen})

	firstBet := &service.SizeBet{
		RoundID:        round.ID,
		UserID:         user.ID,
		Direction:      service.SizeBetDirectionSmall,
		StakeAmount:    5,
		Status:         service.SizeBetStatusPlaced,
		IdempotencyKey: "bet-" + uuid.NewString(),
	}
	require.NoError(t, repo.CreateBetAndDebit(ctx, firstBet, service.NewBetDebitLedger(round, firstBet)))

	secondBet := &service.SizeBet{
		RoundID:        round.ID,
		UserID:         user.ID,
		Direction:      service.SizeBetDirectionBig,
		StakeAmount:    2,
		Status:         service.SizeBetStatusPlaced,
		IdempotencyKey: "bet-" + uuid.NewString(),
	}
	err := repo.CreateBetAndDebit(ctx, secondBet, service.NewBetDebitLedger(round, secondBet))
	require.ErrorIs(t, err, service.ErrSizeBetDuplicateBet)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1`, user.ID).Scan(&balance))
	require.InDelta(t, 15, balance, 0.000001)

	var ledgerCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_wallet_ledger WHERE user_id = $1 AND round_id = $2`, user.ID, round.ID).Scan(&ledgerCount))
	require.Equal(t, 1, ledgerCount)
}

func TestSizeBetRepositoryApplySettlement_RejectsDuplicateSettleWithoutDoubleCredit(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewSizeBetRepository(client, integrationDB).(*sizeBetRepository)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("size-bet-settle-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      8,
	})
	round := mustInsertSizeBetRound(t, ctx, sizeBetRoundInsertInput{status: service.SizeBetRoundStatusOpen})
	betID := mustInsertSizeBet(t, ctx, sizeBetInsertInput{
		roundID:     round.ID,
		userID:      user.ID,
		direction:   service.SizeBetDirectionMid,
		stakeAmount: 2,
		status:      service.SizeBetStatusPlaced,
	})

	bets, err := repo.LoadRoundBetsForSettlement(ctx, round.ID)
	require.NoError(t, err)
	require.Len(t, bets, 1)
	require.Equal(t, betID, bets[0].ID)

	input := service.SettleRoundInput{
		RoundID:         round.ID,
		ResultNumber:    6,
		ResultDirection: service.SizeBetDirectionMid,
		OddsMid:         10,
		SettledAt:       time.Now().UTC(),
		ServerSeed:      "server-seed-" + uuid.NewString(),
	}

	require.NoError(t, repo.ApplySettlement(ctx, input, bets))

	err = repo.ApplySettlement(ctx, input, bets)
	require.ErrorIs(t, err, service.ErrSizeBetRoundAlreadySettled)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1`, user.ID).Scan(&balance))
	require.InDelta(t, 28, balance, 0.000001)

	var payoutCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_wallet_ledger
		WHERE round_id = $1 AND bet_id = $2 AND entry_type = 'bet_payout'
	`, round.ID, betID).Scan(&payoutCount))
	require.Equal(t, 1, payoutCount)

	var roundStatus string
	var resultNumber int
	var resultDirection string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT status, result_number, result_direction
		FROM game_rounds
		WHERE id = $1
	`, round.ID).Scan(&roundStatus, &resultNumber, &resultDirection))
	require.Equal(t, string(service.SizeBetRoundStatusSettled), roundStatus)
	require.Equal(t, 6, resultNumber)
	require.Equal(t, string(service.SizeBetDirectionMid), resultDirection)

	var payoutAmount float64
	var netAmount float64
	var betStatus string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT payout_amount, net_result_amount, status
		FROM game_bets
		WHERE id = $1
	`, betID).Scan(&payoutAmount, &netAmount, &betStatus))
	require.InDelta(t, 20, payoutAmount, 0.000001)
	require.InDelta(t, 18, netAmount, 0.000001)
	require.Equal(t, string(service.SizeBetStatusWon), betStatus)
}

type sizeBetRoundInsertInput struct {
	status service.SizeBetRoundStatus
}

func mustInsertSizeBetRound(t *testing.T, ctx context.Context, input sizeBetRoundInsertInput) *service.SizeBetRound {
	t.Helper()

	if input.status == "" {
		input.status = service.SizeBetRoundStatusOpen
	}

	now := time.Now().UTC()
	allowedStakes, err := json.Marshal([]int{2, 5, 10, 20})
	require.NoError(t, err)

	round := &service.SizeBetRound{
		GameKey:        service.SizeBetGameKey,
		RoundNo:        now.UnixNano(),
		Status:         input.status,
		StartsAt:       now.Add(-30 * time.Second),
		BetClosesAt:    now.Add(30 * time.Second),
		SettlesAt:      now.Add(60 * time.Second),
		ProbSmall:      45,
		ProbMid:        10,
		ProbBig:        45,
		OddsSmall:      2,
		OddsMid:        10,
		OddsBig:        2,
		AllowedStakes:  []int{2, 5, 10, 20},
		ServerSeedHash: "hash-" + uuid.NewString(),
	}

	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO game_rounds (
			game_key, round_no, status, starts_at, bet_closes_at, settles_at,
			prob_small, prob_mid, prob_big, odds_small, odds_mid, odds_big,
			allowed_stakes_json, server_seed_hash
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14
		)
		RETURNING id, created_at, updated_at
	`, round.GameKey, round.RoundNo, round.Status, round.StartsAt, round.BetClosesAt, round.SettlesAt,
		round.ProbSmall, round.ProbMid, round.ProbBig, round.OddsSmall, round.OddsMid, round.OddsBig,
		allowedStakes, round.ServerSeedHash,
	).Scan(&round.ID, &round.CreatedAt, &round.UpdatedAt))

	return round
}

type sizeBetInsertInput struct {
	roundID     int64
	userID      int64
	direction   service.SizeBetDirection
	stakeAmount float64
	status      service.SizeBetStatus
}

func mustInsertSizeBet(t *testing.T, ctx context.Context, input sizeBetInsertInput) int64 {
	t.Helper()

	if input.status == "" {
		input.status = service.SizeBetStatusPlaced
	}

	var betID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO game_bets (
			round_id, user_id, direction, stake_amount, payout_amount,
			net_result_amount, status, idempotency_key
		) VALUES ($1, $2, $3, $4, 0, 0, $5, $6)
		RETURNING id
	`, input.roundID, input.userID, input.direction, input.stakeAmount, input.status, "bet-"+uuid.NewString()).Scan(&betID))
	return betID
}
