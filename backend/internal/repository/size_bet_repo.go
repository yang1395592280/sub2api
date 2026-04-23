package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type sizeBetRepository struct {
	db *sql.DB
}

const leaderboardRefreshSQL = `
WITH affected_users AS (
	SELECT DISTINCT user_id
	FROM game_bets
	WHERE round_id = $1
),
upsert_source AS (
	SELECT
		'all'::varchar(32) AS scope_type,
		'all'::varchar(64) AS scope_key,
		gb.user_id,
		COALESCE(SUM(gb.net_result_amount), 0) AS net_profit,
		COALESCE(SUM(CASE WHEN gb.net_result_amount > 0 THEN 1 ELSE 0 END), 0)::bigint AS win_count,
		COUNT(*)::bigint AS bet_count
	FROM game_bets gb
	JOIN game_rounds gr ON gr.id = gb.round_id
	WHERE gr.game_key = 'size_bet'
		AND gb.user_id IN (SELECT user_id FROM affected_users)
		AND gb.status IN ('won', 'lost', 'refunded')
	GROUP BY gb.user_id
	UNION ALL
	SELECT
		'weekly'::varchar(32),
		to_char(date_trunc('week', gr.starts_at AT TIME ZONE 'UTC'), 'YYYY-MM-DD')::varchar(64),
		gb.user_id,
		COALESCE(SUM(gb.net_result_amount), 0),
		COALESCE(SUM(CASE WHEN gb.net_result_amount > 0 THEN 1 ELSE 0 END), 0)::bigint,
		COUNT(*)::bigint
	FROM game_bets gb
	JOIN game_rounds gr ON gr.id = gb.round_id
	WHERE gr.game_key = 'size_bet'
		AND gb.user_id IN (SELECT user_id FROM affected_users)
		AND gb.status IN ('won', 'lost', 'refunded')
	GROUP BY gb.user_id, date_trunc('week', gr.starts_at AT TIME ZONE 'UTC')
)
INSERT INTO game_rank_snapshots (
	scope_type, scope_key, user_id, net_profit, win_count, bet_count, updated_at
)
SELECT scope_type, scope_key, user_id, net_profit, win_count, bet_count, NOW()
FROM upsert_source
ON CONFLICT (scope_type, scope_key, user_id)
DO UPDATE SET
	net_profit = EXCLUDED.net_profit,
	win_count = EXCLUDED.win_count,
	bet_count = EXCLUDED.bet_count,
	updated_at = NOW()
`

func NewSizeBetRepository(_ *dbent.Client, sqlDB *sql.DB) service.SizeBetRepository {
	return &sizeBetRepository{db: sqlDB}
}

func (r *sizeBetRepository) GetRoundByID(ctx context.Context, roundID int64) (*service.SizeBetRound, error) {
	return r.queryRound(ctx, `SELECT `+sizeBetRoundColumns+` FROM game_rounds WHERE id = $1 AND game_key = $2`, roundID, service.SizeBetGameKey)
}

func (r *sizeBetRepository) GetRoundByTime(ctx context.Context, now time.Time) (*service.SizeBetRound, error) {
	return r.queryRound(ctx, `SELECT `+sizeBetRoundColumns+` FROM game_rounds
		WHERE game_key = $1 AND starts_at <= $2 AND settles_at > $2
		ORDER BY starts_at DESC LIMIT 1`, service.SizeBetGameKey, now)
}

func (r *sizeBetRepository) CreateRound(ctx context.Context, round *service.SizeBetRound) (*service.SizeBetRound, error) {
	stakesJSON, err := json.Marshal(round.AllowedStakes)
	if err != nil {
		return nil, err
	}
	created, err := scanSizeBetRound(r.db.QueryRowContext(ctx, `
		INSERT INTO game_rounds (
			game_key, round_no, status, starts_at, bet_closes_at, settles_at,
			prob_small, prob_mid, prob_big, odds_small, odds_mid, odds_big,
			allowed_stakes_json, server_seed_hash, server_seed
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15
		)
		ON CONFLICT (round_no) DO NOTHING
		RETURNING `+sizeBetRoundColumns,
		round.GameKey, round.RoundNo, round.Status, round.StartsAt, round.BetClosesAt, round.SettlesAt,
		round.ProbSmall, round.ProbMid, round.ProbBig, round.OddsSmall, round.OddsMid, round.OddsBig,
		stakesJSON, round.ServerSeedHash, round.ServerSeed,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return r.queryRound(ctx, `SELECT `+sizeBetRoundColumns+` FROM game_rounds WHERE round_no = $1`, round.RoundNo)
	}
	return created, err
}

func (r *sizeBetRepository) CreateBetAndDebit(ctx context.Context, bet *service.SizeBet, entry *service.SizeBetLedgerEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	before, after, err := debitUserBalance(ctx, tx, bet.UserID, bet.StakeAmount)
	if err != nil {
		return err
	}
	if err := insertBet(ctx, tx, bet); err != nil {
		return err
	}
	if entry != nil {
		entry.BetID = int64Ptr(bet.ID)
		entry.BalanceBefore = before
		entry.BalanceAfter = after
		if err := insertLedger(ctx, tx, entry); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *sizeBetRepository) LoadRoundBetsForSettlement(ctx context.Context, roundID int64) ([]service.SizeBet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, round_id, user_id, direction, stake_amount, payout_amount,
		       net_result_amount, status, idempotency_key, placed_at, settled_at
		FROM game_bets
		WHERE round_id = $1 AND status = $2
		ORDER BY id ASC
	`, roundID, service.SizeBetStatusPlaced)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bets []service.SizeBet
	for rows.Next() {
		bet, err := scanSizeBet(rows)
		if err != nil {
			return nil, err
		}
		bets = append(bets, *bet)
	}
	return bets, rows.Err()
}

func (r *sizeBetRepository) ApplySettlement(ctx context.Context, input service.SettleRoundInput, bets []service.SizeBet) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_rounds
		SET status = $2, result_number = $3, result_direction = $4,
		    server_seed = CASE WHEN $5 = '' THEN server_seed ELSE $5 END,
		    updated_at = NOW()
		WHERE id = $1 AND game_key = $6
	`, input.RoundID, service.SizeBetRoundStatusSettled, input.ResultNumber, input.ResultDirection, input.ServerSeed, service.SizeBetGameKey); err != nil {
		return err
	}

	round := &service.SizeBetRound{ID: input.RoundID, GameKey: service.SizeBetGameKey}
	for _, bet := range bets {
		payout := 0.0
		status := service.SizeBetStatusLost
		if bet.Direction == input.ResultDirection {
			payout = bet.StakeAmount * input.OddsFor(bet.Direction)
			status = service.SizeBetStatusWon
		}
		net := payout - bet.StakeAmount
		if _, err := tx.ExecContext(ctx, `
			UPDATE game_bets
			SET payout_amount = $1, net_result_amount = $2, status = $3, settled_at = $4
			WHERE id = $5 AND status = $6
		`, payout, net, status, input.SettledAt, bet.ID, service.SizeBetStatusPlaced); err != nil {
			return err
		}
		if payout <= 0 {
			continue
		}
		before, after, err := creditUserBalance(ctx, tx, bet.UserID, payout)
		if err != nil {
			return err
		}
		settledBet := bet
		settledBet.PayoutAmount = payout
		settledBet.NetResultAmount = net
		settledBet.Status = status
		settledBet.SettledAt = &input.SettledAt
		entry := service.NewBetPayoutLedger(round, &settledBet, input.OddsFor(bet.Direction))
		entry.BalanceBefore = before
		entry.BalanceAfter = after
		if err := insertLedger(ctx, tx, entry); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *sizeBetRepository) RefreshLeaderboardSnapshots(ctx context.Context, settledRoundID int64) error {
	_, err := r.db.ExecContext(ctx, leaderboardRefreshSQL, settledRoundID)
	return err
}

const sizeBetRoundColumns = `id, game_key, round_no, status, starts_at, bet_closes_at, settles_at,
	prob_small, prob_mid, prob_big, odds_small, odds_mid, odds_big, allowed_stakes_json,
	result_number, result_direction, server_seed_hash, COALESCE(server_seed, ''), created_at, updated_at`

func (r *sizeBetRepository) queryRound(ctx context.Context, query string, args ...any) (*service.SizeBetRound, error) {
	round, err := scanSizeBetRound(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return round, err
}

func scanSizeBetRound(row interface{ Scan(dest ...any) error }) (*service.SizeBetRound, error) {
	var round service.SizeBetRound
	var stakesJSON []byte
	var resultNumber sql.NullInt64
	var resultDirection sql.NullString
	if err := row.Scan(
		&round.ID, &round.GameKey, &round.RoundNo, &round.Status, &round.StartsAt, &round.BetClosesAt, &round.SettlesAt,
		&round.ProbSmall, &round.ProbMid, &round.ProbBig, &round.OddsSmall, &round.OddsMid, &round.OddsBig, &stakesJSON,
		&resultNumber, &resultDirection, &round.ServerSeedHash, &round.ServerSeed, &round.CreatedAt, &round.UpdatedAt,
	); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(stakesJSON, &round.AllowedStakes)
	if resultNumber.Valid {
		v := int(resultNumber.Int64)
		round.ResultNumber = &v
	}
	if resultDirection.Valid {
		round.ResultDirection = service.SizeBetDirection(resultDirection.String)
	}
	return &round, nil
}

func scanSizeBet(rows interface{ Scan(dest ...any) error }) (*service.SizeBet, error) {
	var bet service.SizeBet
	var settledAt sql.NullTime
	if err := rows.Scan(
		&bet.ID, &bet.RoundID, &bet.UserID, &bet.Direction, &bet.StakeAmount, &bet.PayoutAmount,
		&bet.NetResultAmount, &bet.Status, &bet.IdempotencyKey, &bet.PlacedAt, &settledAt,
	); err != nil {
		return nil, err
	}
	if settledAt.Valid {
		bet.SettledAt = &settledAt.Time
	}
	return &bet, nil
}

func debitUserBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, float64, error) {
	var before, after float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance + $1, balance
	`, amount, userID).Scan(&before, &after)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, missingUserOrBalance(ctx, tx, userID)
	}
	return before, after, err
}

func creditUserBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, float64, error) {
	var before, after float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance + $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance - $1, balance
	`, amount, userID).Scan(&before, &after)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, service.ErrUserNotFound
	}
	return before, after, err
}

func missingUserOrBalance(ctx context.Context, tx *sql.Tx, userID int64) error {
	var exists int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL`, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrUserNotFound
	}
	if err != nil {
		return err
	}
	return service.ErrInsufficientBalance
}

func insertBet(ctx context.Context, tx *sql.Tx, bet *service.SizeBet) error {
	return tx.QueryRowContext(ctx, `
		INSERT INTO game_bets (
			round_id, user_id, direction, stake_amount, payout_amount, net_result_amount,
			status, idempotency_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, placed_at
	`, bet.RoundID, bet.UserID, bet.Direction, bet.StakeAmount, bet.PayoutAmount,
		bet.NetResultAmount, bet.Status, bet.IdempotencyKey).Scan(&bet.ID, &bet.PlacedAt)
}

func insertLedger(ctx context.Context, tx *sql.Tx, entry *service.SizeBetLedgerEntry) error {
	return tx.QueryRowContext(ctx, `
		INSERT INTO game_wallet_ledger (
			user_id, game_key, round_id, bet_id, entry_type, direction, stake_amount,
			delta_amount, balance_before, balance_after, reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at
	`, entry.UserID, entry.GameKey, entry.RoundID, entry.BetID, entry.EntryType, entry.Direction,
		entry.StakeAmount, entry.DeltaAmount, entry.BalanceBefore, entry.BalanceAfter, entry.Reason,
	).Scan(&entry.ID, &entry.CreatedAt)
}

func int64Ptr(v int64) *int64 {
	return &v
}
