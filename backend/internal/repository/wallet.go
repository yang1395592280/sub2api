package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func lockWallet(ctx context.Context, q sqlExecutor, userID int64) (service.WalletBalance, error) {
	var w service.WalletBalance
	rows, err := q.QueryContext(ctx, `SELECT balance, checkin_balance, liyu_balance, gift_balance
		FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, userID)
	if err != nil {
		return w, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return w, err
		}
		return w, service.ErrUserNotFound
	}
	err = rows.Scan(&w.Paid, &w.Checkin, &w.Liyu, &w.Gift)
	return w, err
}

// applyWalletDelta and its audit entry must share the caller's transaction.
func applyWalletDelta(ctx context.Context, q sqlExecutor, userID int64, delta service.WalletBalance, event, operation string, apiKeyID int64, actorID *int64, notes string) (service.WalletBalance, error) {
	var after service.WalletBalance
	rows, err := q.QueryContext(ctx, `UPDATE users SET balance = balance + $1,
		checkin_balance = checkin_balance + $2, liyu_balance = liyu_balance + $3,
		gift_balance = gift_balance + $4, updated_at = NOW()
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING balance, checkin_balance, liyu_balance, gift_balance`,
		delta.Paid, delta.Checkin, delta.Liyu, delta.Gift, userID)
	if err != nil {
		return after, err
	}
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		if err == nil {
			err = service.ErrUserNotFound
		}
		return after, err
	}
	err = rows.Scan(&after.Paid, &after.Checkin, &after.Liyu, &after.Gift)
	closeErr := rows.Close()
	if err != nil {
		return after, err
	}
	if closeErr != nil {
		return after, closeErr
	}
	_, err = q.ExecContext(ctx, `INSERT INTO wallet_transactions
		(user_id, event_type, operation_id, api_key_id, paid_delta, checkin_delta, liyu_delta, gift_delta,
		 paid_after, checkin_after, liyu_after, gift_after, actor_id, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		userID, event, operation, apiKeyID, delta.Paid, delta.Checkin, delta.Liyu, delta.Gift,
		after.Paid, after.Checkin, after.Liyu, after.Gift, actorID, notes)
	return after, err
}

func chargeWallet(ctx context.Context, q sqlExecutor, userID int64, amount float64, requestID string, apiKeyID int64) (service.WalletBalance, service.WalletBalance, bool, error) {
	before, err := lockWallet(ctx, q, userID)
	if err != nil {
		return before, service.WalletBalance{}, false, err
	}
	debit, err := before.Split(amount)
	if err != nil {
		return before, debit, false, err
	}
	delta := service.WalletBalance{Paid: -debit.Paid, Checkin: -debit.Checkin, Liyu: -debit.Liyu, Gift: -debit.Gift}
	after, err := applyWalletDelta(ctx, q, userID, delta, "usage", requestID, apiKeyID, nil, "")
	return after, debit, before.Paid >= debit.Paid, err
}

func scanWalletHold(ctx context.Context, tx *sql.Tx, batchID string, userID int64) (service.WalletBalance, bool, error) {
	var w service.WalletBalance
	var settled bool
	err := tx.QueryRowContext(ctx, `SELECT paid, checkin, liyu, gift, settled FROM wallet_image_holds
		WHERE batch_id=$1 AND user_id=$2 FOR UPDATE`, batchID, userID).
		Scan(&w.Paid, &w.Checkin, &w.Liyu, &w.Gift, &settled)
	if errors.Is(err, sql.ErrNoRows) {
		return w, false, nil
	}
	if err == nil && settled {
		return w, true, errors.New("image wallet hold already settled")
	}
	return w, err == nil, err
}
