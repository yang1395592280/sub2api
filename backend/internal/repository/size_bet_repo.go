package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
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
		AND gb.status IN ('won', 'lost')
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
		AND gb.status IN ('won', 'lost')
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

	if err := ensureRoundBettableForWrite(ctx, tx, bet.RoundID); err != nil {
		return err
	}
	before, after, err := debitUserBalance(ctx, tx, bet.UserID, bet.StakeAmount)
	if err != nil {
		return err
	}
	if err := insertBet(ctx, tx, bet); err != nil {
		return translateSizeBetWriteError(err)
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

func (r *sizeBetRepository) GetBetByRoundAndUser(ctx context.Context, roundID, userID int64) (*service.SizeBet, error) {
	bet, err := scanSizeBet(r.db.QueryRowContext(ctx, `
		SELECT id, round_id, user_id, direction, stake_amount, payout_amount,
		       net_result_amount, status, idempotency_key, placed_at, settled_at
		FROM game_bets
		WHERE round_id = $1 AND user_id = $2
		ORDER BY id DESC
		LIMIT 1
	`, roundID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return bet, err
}

func (r *sizeBetRepository) ListRecentRounds(ctx context.Context, limit int) ([]service.SizeBetRound, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+sizeBetRoundColumns+`
		FROM game_rounds
		WHERE game_key = $1 AND status = $2
		ORDER BY starts_at DESC
		LIMIT $3
	`, service.SizeBetGameKey, service.SizeBetRoundStatusSettled, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSizeBetRounds(rows)
}

func (r *sizeBetRepository) ListUserHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.SizeBetUserHistoryItem, *pagination.PaginationResult, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_bets gb
		JOIN game_rounds gr ON gr.id = gb.round_id
		WHERE gr.game_key = $1 AND gb.user_id = $2
	`, service.SizeBetGameKey, userID).Scan(&total); err != nil {
		return nil, nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			gb.id, gb.round_id, gr.round_no, gb.direction, gb.stake_amount, gb.payout_amount,
			gb.net_result_amount, gb.status, gb.idempotency_key, gb.placed_at, gb.settled_at,
			gr.result_number, gr.result_direction, gr.starts_at, gr.settles_at
		FROM game_bets gb
		JOIN game_rounds gr ON gr.id = gb.round_id
		WHERE gr.game_key = $1 AND gb.user_id = $2
		ORDER BY gb.placed_at DESC
		LIMIT $3 OFFSET $4
	`, service.SizeBetGameKey, userID, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.SizeBetUserHistoryItem, 0)
	for rows.Next() {
		var item service.SizeBetUserHistoryItem
		var settledAt sql.NullTime
		var resultNumber sql.NullInt64
		var resultDirection sql.NullString
		if err := rows.Scan(
			&item.BetID, &item.RoundID, &item.RoundNo, &item.Direction, &item.StakeAmount, &item.PayoutAmount,
			&item.NetResultAmount, &item.Status, &item.IdempotencyKey, &item.PlacedAt, &settledAt,
			&resultNumber, &resultDirection, &item.RoundStartsAt, &item.RoundSettlesAt,
		); err != nil {
			return nil, nil, err
		}
		if settledAt.Valid {
			item.SettledAt = &settledAt.Time
		}
		if resultNumber.Valid {
			v := int(resultNumber.Int64)
			item.ResultNumber = &v
		}
		if resultDirection.Valid {
			item.ResultDirection = service.SizeBetDirection(resultDirection.String)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, buildSizeBetPaginationResult(total, params), nil
}

func (r *sizeBetRepository) ListLeaderboard(ctx context.Context, scopeType, scopeKey string, limit int) ([]service.SizeBetLeaderboardEntry, time.Time, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			grs.user_id,
			COALESCE(NULLIF(u.username, ''), ''),
			grs.net_profit,
			grs.win_count,
			grs.bet_count,
			grs.updated_at
		FROM game_rank_snapshots grs
		LEFT JOIN users u ON u.id = grs.user_id
		WHERE grs.scope_type = $1 AND grs.scope_key = $2 AND grs.bet_count > 0
		ORDER BY grs.net_profit DESC, grs.win_count DESC, grs.bet_count ASC, grs.user_id ASC
		LIMIT $3
	`, scopeType, scopeKey, limit)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()

	items := make([]service.SizeBetLeaderboardEntry, 0)
	var refreshedAt time.Time
	for rows.Next() {
		var item service.SizeBetLeaderboardEntry
		if err := rows.Scan(&item.UserID, &item.Username, &item.NetProfit, &item.WinCount, &item.BetCount, &refreshedAt); err != nil {
			return nil, time.Time{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, err
	}
	return items, refreshedAt, nil
}

func (r *sizeBetRepository) ListAdminRounds(ctx context.Context, params pagination.PaginationParams) ([]service.SizeBetRound, *pagination.PaginationResult, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_rounds
		WHERE game_key = $1
	`, service.SizeBetGameKey).Scan(&total); err != nil {
		return nil, nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT `+sizeBetRoundColumns+`
		FROM game_rounds
		WHERE game_key = $1
		ORDER BY starts_at DESC
		LIMIT $2 OFFSET $3
	`, service.SizeBetGameKey, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items, err := scanSizeBetRounds(rows)
	if err != nil {
		return nil, nil, err
	}
	return items, buildSizeBetPaginationResult(total, params), nil
}

func (r *sizeBetRepository) ListAdminBets(ctx context.Context, params pagination.PaginationParams, filter service.SizeBetAdminBetFilter) ([]service.SizeBetAdminBet, *pagination.PaginationResult, error) {
	where, args := buildSizeBetBetFilters(filter)

	var total int64
	countQuery := `SELECT COUNT(*) FROM game_bets gb JOIN game_rounds gr ON gr.id = gb.round_id WHERE ` + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			gb.id, gb.round_id, gr.round_no, gb.user_id,
			COALESCE(NULLIF(u.username, ''), ''),
			gb.direction, gb.stake_amount, gb.payout_amount, gb.net_result_amount,
			gb.status, gb.idempotency_key, gb.placed_at, gb.settled_at
		FROM game_bets gb
		JOIN game_rounds gr ON gr.id = gb.round_id
		LEFT JOIN users u ON u.id = gb.user_id
		WHERE `+where+`
		ORDER BY gb.placed_at DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)),
		args...,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.SizeBetAdminBet, 0)
	for rows.Next() {
		var item service.SizeBetAdminBet
		var settledAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.RoundID, &item.RoundNo, &item.UserID, &item.Username,
			&item.Direction, &item.StakeAmount, &item.PayoutAmount, &item.NetResultAmount,
			&item.Status, &item.IdempotencyKey, &item.PlacedAt, &settledAt,
		); err != nil {
			return nil, nil, err
		}
		if settledAt.Valid {
			item.SettledAt = &settledAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, buildSizeBetPaginationResult(total, params), nil
}

func (r *sizeBetRepository) ListAdminLedger(ctx context.Context, params pagination.PaginationParams, filter service.SizeBetAdminLedgerFilter) ([]service.SizeBetLedgerEntry, *pagination.PaginationResult, error) {
	where, args := buildSizeBetLedgerFilters(filter)

	var total int64
	countQuery := `SELECT COUNT(*) FROM game_wallet_ledger gl WHERE ` + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, user_id, game_key, round_id, bet_id, entry_type, direction,
			stake_amount, delta_amount, balance_before, balance_after, reason, created_at
		FROM game_wallet_ledger gl
		WHERE `+where+`
		ORDER BY created_at DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)),
		args...,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.SizeBetLedgerEntry, 0)
	for rows.Next() {
		var item service.SizeBetLedgerEntry
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.GameKey, &item.RoundID, &item.BetID, &item.EntryType,
			&item.Direction, &item.StakeAmount, &item.DeltaAmount, &item.BalanceBefore,
			&item.BalanceAfter, &item.Reason, &item.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, buildSizeBetPaginationResult(total, params), nil
}

func (r *sizeBetRepository) RefundRound(ctx context.Context, roundID int64, refundedAt time.Time) ([]service.SizeBet, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	var resultNumber sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, result_number
		FROM game_rounds
		WHERE id = $1 AND game_key = $2
		FOR UPDATE
	`, roundID, service.SizeBetGameKey).Scan(&status, &resultNumber); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSizeBetRoundNotFound
	} else if err != nil {
		return nil, err
	}
	if status == string(service.SizeBetRoundStatusSettled) && resultNumber.Valid {
		return nil, service.ErrSizeBetRoundAlreadySettled
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, round_id, user_id, direction, stake_amount, payout_amount,
		       net_result_amount, status, idempotency_key, placed_at, settled_at
		FROM game_bets
		WHERE round_id = $1 AND status = $2
		ORDER BY id ASC
		FOR UPDATE
	`, roundID, service.SizeBetStatusPlaced)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refunded := make([]service.SizeBet, 0)
	for rows.Next() {
		bet, scanErr := scanSizeBet(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		before, after, creditErr := creditUserBalance(ctx, tx, bet.UserID, bet.StakeAmount)
		if creditErr != nil {
			return nil, creditErr
		}
		res, updateErr := tx.ExecContext(ctx, `
			UPDATE game_bets
			SET payout_amount = $1, net_result_amount = 0, status = $2, settled_at = $3
			WHERE id = $4 AND status = $5
		`, bet.StakeAmount, service.SizeBetStatusRefunded, refundedAt, bet.ID, service.SizeBetStatusPlaced)
		if updateErr != nil {
			return nil, updateErr
		}
		affected, affectedErr := res.RowsAffected()
		if affectedErr != nil {
			return nil, affectedErr
		}
		if affected != 1 {
			return nil, service.ErrSizeBetSettlementConflict
		}

		entry := &service.SizeBetLedgerEntry{
			UserID:        bet.UserID,
			GameKey:       service.SizeBetGameKey,
			RoundID:       int64Ptr(roundID),
			BetID:         int64Ptr(bet.ID),
			EntryType:     "bet_refund",
			Direction:     string(bet.Direction),
			StakeAmount:   bet.StakeAmount,
			DeltaAmount:   bet.StakeAmount,
			BalanceBefore: before,
			BalanceAfter:  after,
			Reason:        "size bet refunded",
		}
		if insertErr := insertLedger(ctx, tx, entry); insertErr != nil {
			return nil, insertErr
		}

		bet.PayoutAmount = bet.StakeAmount
		bet.NetResultAmount = 0
		bet.Status = service.SizeBetStatusRefunded
		bet.SettledAt = &refundedAt
		refunded = append(refunded, *bet)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_rounds
		SET status = $2, result_number = NULL, result_direction = '', updated_at = NOW()
		WHERE id = $1 AND game_key = $3
	`, roundID, service.SizeBetRoundStatusSettled, service.SizeBetGameKey); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return refunded, nil
}

func (r *sizeBetRepository) ListRoundsDueForSettlement(ctx context.Context, now time.Time, limit int) ([]service.SizeBetRound, error) {
	if limit <= 0 {
		limit = 8
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+sizeBetRoundColumns+`
		FROM game_rounds
		WHERE game_key = $1 AND status = $2 AND settles_at <= $3
		ORDER BY settles_at ASC
		LIMIT $4
	`, service.SizeBetGameKey, service.SizeBetRoundStatusOpen, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSizeBetRounds(rows)
}

func (r *sizeBetRepository) LoadRoundBetsForSettlement(ctx context.Context, roundID int64) ([]service.SizeBet, error) {
	return loadRoundBetsForSettlementQuerier(ctx, r.db, roundID)
}

func (r *sizeBetRepository) ApplySettlement(ctx context.Context, input service.SettleRoundInput) ([]service.SizeBet, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := claimSizeBetRoundForSettlement(ctx, tx, input); err != nil {
		return nil, err
	}

	bets, err := loadRoundBetsForSettlementQuerier(ctx, tx, input.RoundID)
	if err != nil {
		return nil, err
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
		res, err := tx.ExecContext(ctx, `
			UPDATE game_bets
			SET payout_amount = $1, net_result_amount = $2, status = $3, settled_at = $4
			WHERE id = $5 AND status = $6
		`, payout, net, status, input.SettledAt, bet.ID, service.SizeBetStatusPlaced)
		if err != nil {
			return nil, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected != 1 {
			return nil, service.ErrSizeBetSettlementConflict
		}
		if payout <= 0 {
			continue
		}
		before, after, err := creditUserBalance(ctx, tx, bet.UserID, payout)
		if err != nil {
			return nil, err
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
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return bets, nil
}

func claimSizeBetRoundForSettlement(ctx context.Context, tx *sql.Tx, input service.SettleRoundInput) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE game_rounds
		SET status = $2, result_number = $3, result_direction = $4,
		    server_seed = CASE WHEN $5 = '' THEN server_seed ELSE $5 END,
		    updated_at = NOW()
		WHERE id = $1 AND game_key = $6 AND status = $7
	`, input.RoundID, service.SizeBetRoundStatusSettled, input.ResultNumber, input.ResultDirection, input.ServerSeed, service.SizeBetGameKey, service.SizeBetRoundStatusOpen)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}

	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT status
		FROM game_rounds
		WHERE id = $1 AND game_key = $2
	`, input.RoundID, service.SizeBetGameKey).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrSizeBetRoundNotFound
	}
	if err != nil {
		return err
	}
	if status == string(service.SizeBetRoundStatusSettled) {
		return service.ErrSizeBetRoundAlreadySettled
	}
	return service.ErrSizeBetSettlementConflict
}

func ensureRoundBettableForWrite(ctx context.Context, tx *sql.Tx, roundID int64) error {
	var status string
	var betClosesAt time.Time
	var dbNow time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT status, bet_closes_at, NOW()
		FROM game_rounds
		WHERE id = $1 AND game_key = $2
		FOR UPDATE
	`, roundID, service.SizeBetGameKey).Scan(&status, &betClosesAt, &dbNow)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrSizeBetRoundNotFound
	}
	if err != nil {
		return err
	}
	if status != string(service.SizeBetRoundStatusOpen) || dbNow.After(betClosesAt) {
		return service.ErrSizeBetClosed
	}
	return nil
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

func scanSizeBetRounds(rows *sql.Rows) ([]service.SizeBetRound, error) {
	items := make([]service.SizeBetRound, 0)
	for rows.Next() {
		round, err := scanSizeBetRound(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *round)
	}
	return items, rows.Err()
}

type sizeBetBetRowsQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func loadRoundBetsForSettlementQuerier(ctx context.Context, querier sizeBetBetRowsQuerier, roundID int64) ([]service.SizeBet, error) {
	rows, err := querier.QueryContext(ctx, `
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

func translateSizeBetWriteError(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}
	if pqErr.Code != "23505" {
		return err
	}
	switch pqErr.Constraint {
	case "idx_game_bets_round_user", "idx_game_bets_idempotency_key":
		return service.ErrSizeBetDuplicateBet
	default:
		return err
	}
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

func buildSizeBetPaginationResult(total int64, params pagination.PaginationParams) *pagination.PaginationResult {
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.Limit()
	pages := 1
	if total > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &pagination.PaginationResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	}
}

func buildSizeBetBetFilters(filter service.SizeBetAdminBetFilter) (string, []any) {
	clauses := []string{"gr.game_key = $1"}
	args := []any{service.SizeBetGameKey}
	next := 2

	if filter.RoundID != nil {
		clauses = append(clauses, fmt.Sprintf("gb.round_id = $%d", next))
		args = append(args, *filter.RoundID)
		next++
	}
	if filter.UserID != nil {
		clauses = append(clauses, fmt.Sprintf("gb.user_id = $%d", next))
		args = append(args, *filter.UserID)
		next++
	}
	if filter.Status != "" {
		clauses = append(clauses, fmt.Sprintf("gb.status = $%d", next))
		args = append(args, filter.Status)
	}
	return strings.Join(clauses, " AND "), args
}

func buildSizeBetLedgerFilters(filter service.SizeBetAdminLedgerFilter) (string, []any) {
	clauses := []string{"gl.game_key = $1"}
	args := []any{service.SizeBetGameKey}
	next := 2

	if filter.RoundID != nil {
		clauses = append(clauses, fmt.Sprintf("gl.round_id = $%d", next))
		args = append(args, *filter.RoundID)
		next++
	}
	if filter.UserID != nil {
		clauses = append(clauses, fmt.Sprintf("gl.user_id = $%d", next))
		args = append(args, *filter.UserID)
		next++
	}
	if filter.EntryType != "" {
		clauses = append(clauses, fmt.Sprintf("gl.entry_type = $%d", next))
		args = append(args, filter.EntryType)
	}
	return strings.Join(clauses, " AND "), args
}
