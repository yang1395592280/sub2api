package repository

import (
	"context"
	"fmt"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *userRepository) RefundWallet(ctx context.Context, userID int64, paid, welfare float64, source string, actorID int64, notes, operationID string) error {
	for _, amount := range []float64{paid, welfare} {
		if amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
			return infraerrors.BadRequest("INVALID_WALLET_REFUND", "退款金额无效")
		}
	}
	paid = service.QuantizeUsageBillingAmount(paid)
	welfare = service.QuantizeUsageBillingAmount(welfare)
	if paid == 0 && welfare == 0 {
		return infraerrors.BadRequest("INVALID_WALLET_REFUND", "退款金额至少一项大于0")
	}
	delta := service.WalletBalance{Paid: -paid}
	if welfare > 0 {
		switch source {
		case "checkin":
			delta.Checkin = -welfare
		case "liyu":
			delta.Liyu = -welfare
		case "gift":
			delta.Gift = -welfare
		default:
			return infraerrors.BadRequest("INVALID_WELFARE_SOURCE", "请选择要退款的福利来源")
		}
	}
	if operationID == "" {
		operationID = fmt.Sprintf("%d:%d", userID, time.Now().UnixNano())
	}
	refund := func(q sqlExecutor) error {
		before, err := lockWallet(ctx, q, userID)
		if err != nil {
			return err
		}
		rows, err := q.QueryContext(ctx, `SELECT paid_delta,checkin_delta,liyu_delta,gift_delta,notes FROM wallet_transactions WHERE user_id=$1 AND event_type='admin_refund' AND operation_id=$2 AND api_key_id=0`, userID, operationID)
		if err != nil {
			return err
		}
		if rows.Next() {
			var previous service.WalletBalance
			var previousNotes string
			err = rows.Scan(&previous.Paid, &previous.Checkin, &previous.Liyu, &previous.Gift, &previousNotes)
			rows.Close()
			if err != nil {
				return err
			}
			if previous != delta || previousNotes != notes {
				return service.ErrIdempotencyKeyConflict
			}
			return nil
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if paid > 0 && paid > before.Paid {
			return infraerrors.BadRequest("INSUFFICIENT_BALANCE", "充值余额不足，无法退款")
		}
		// Never substitute another welfare source or erase an existing negative balance.
		for _, pair := range [][2]float64{{before.Checkin, -delta.Checkin}, {before.Liyu, -delta.Liyu}, {before.Gift, -delta.Gift}} {
			if pair[1] > 0 && pair[1] > pair[0] {
				return infraerrors.BadRequest("INSUFFICIENT_WELFARE_BALANCE", "所选福利余额不足，无法退款")
			}
		}
		var actor *int64
		if actorID > 0 {
			actor = &actorID
		}
		_, err = applyWalletDelta(ctx, q, userID, delta, "admin_refund", operationID, 0, actor, notes)
		return err
	}
	client := clientFromContext(ctx, r.client)
	if dbent.TxFromContext(ctx) != nil {
		return refund(client)
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := refund(tx.Client()); err != nil {
		return err
	}
	return tx.Commit()
}

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
