package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *userRepository) CreditWallet(ctx context.Context, userID int64, paid, gift float64, actorID int64, notes, operationID string) (bool, error) {
	if operationID == "" {
		operationID = fmt.Sprintf("%d:%d", userID, time.Now().UnixNano())
	}
	applied := false
	credit := func(q sqlExecutor) error {
		if _, err := lockWallet(ctx, q, userID); err != nil {
			return err
		}
		rows, err := q.QueryContext(ctx, `SELECT paid_delta,gift_delta,notes FROM wallet_transactions WHERE user_id=$1 AND event_type='admin_credit' AND operation_id=$2 AND api_key_id=0`, userID, operationID)
		if err != nil {
			return err
		}
		if rows.Next() {
			var previousPaid, previousGift float64
			var previousNotes string
			err = rows.Scan(&previousPaid, &previousGift, &previousNotes)
			rows.Close()
			if err != nil {
				return err
			}
			if previousPaid != paid || previousGift != gift || previousNotes != notes {
				return service.ErrIdempotencyKeyConflict
			}
			return nil
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		var actor *int64
		if actorID > 0 {
			actor = &actorID
		}
		_, err = applyWalletDelta(ctx, q, userID, service.WalletBalance{Paid: paid, Gift: gift}, "admin_credit", operationID, 0, actor, notes)
		if err == nil && paid > 0 {
			// Percentage-based balance alerts use cumulative paid credit, excluding gifts.
			_, err = q.ExecContext(ctx, `UPDATE users SET total_recharged=total_recharged+$1 WHERE id=$2`, paid, userID)
		}
		applied = err == nil
		return err
	}
	client := clientFromContext(ctx, r.client)
	if dbent.TxFromContext(ctx) != nil {
		err := credit(client)
		return applied, err
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := credit(tx.Client()); err != nil {
		return false, err
	}
	err = tx.Commit()
	return applied && err == nil, err
}
