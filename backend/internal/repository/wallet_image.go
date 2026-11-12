package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func reserveWalletImage(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	before, err := lockWallet(ctx, tx, cmd.UserID)
	if err != nil {
		return nil, err
	}
	if before.Available() < cmd.HoldAmount {
		return nil, service.ErrBatchImageInsufficientBalance
	}
	held, err := before.Split(cmd.HoldAmount)
	if err != nil {
		return nil, err
	}
	after, err := applyWalletDelta(ctx, tx, cmd.UserID,
		service.WalletBalance{Paid: -held.Paid, Checkin: -held.Checkin, Liyu: -held.Liyu, Gift: -held.Gift},
		"image_hold", cmd.BatchID, cmd.APIKeyID, nil, "")
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO wallet_image_holds(batch_id,user_id,paid,checkin,liyu,gift)
		VALUES($1,$2,$3,$4,$5,$6)`, cmd.BatchID, cmd.UserID, held.Paid, held.Checkin, held.Liyu, held.Gift)
	if err != nil {
		return nil, err
	}
	var frozen float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET frozen_balance=frozen_balance+$1 WHERE id=$2 RETURNING frozen_balance`, cmd.HoldAmount, cmd.UserID).Scan(&frozen)
	return &service.BatchImageBalanceHoldResult{NewBalance: &after.Paid, FrozenBalance: &frozen}, err
}

// settleWalletImage returns found=false for jobs reserved before the migration;
// those retain the existing paid-only release/capture implementation.
func settleWalletImage(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand, release bool) (*service.BatchImageBalanceHoldResult, bool, error) {
	if _, err := lockWallet(ctx, tx, cmd.UserID); err != nil {
		return nil, false, err
	}
	held, found, err := scanWalletHold(ctx, tx, cmd.BatchID, cmd.UserID)
	if err != nil || !found {
		return nil, found, err
	}
	if service.QuantizeUsageBillingAmount(held.Total()) != service.QuantizeUsageBillingAmount(cmd.HoldAmount) {
		return nil, true, errors.New("image wallet hold amount mismatch")
	}
	cost := cmd.ActualAmount
	if release {
		cost = 0
	}
	if cost < 0 || cost > held.Total() {
		return nil, true, service.ErrBatchImageSettlementCostExceedsHold
	}
	debit, err := held.Split(cost)
	if err != nil {
		return nil, true, err
	}
	refund := service.WalletBalance{Paid: held.Paid - debit.Paid, Checkin: held.Checkin - debit.Checkin, Liyu: held.Liyu - debit.Liyu, Gift: held.Gift - debit.Gift}
	after, err := applyWalletDelta(ctx, tx, cmd.UserID, held, "image_release", cmd.BatchID, cmd.APIKeyID, nil, "")
	if err != nil {
		return nil, true, err
	}
	if !release {
		after, err = applyWalletDelta(ctx, tx, cmd.UserID, service.WalletBalance{Paid: -debit.Paid, Checkin: -debit.Checkin, Liyu: -debit.Liyu, Gift: -debit.Gift}, "usage", cmd.RequestID, cmd.APIKeyID, nil, "")
		if err != nil {
			return nil, true, err
		}
	}
	if refund.Checkin > 0 {
		var caseID sql.NullInt64
		err = tx.QueryRowContext(ctx, `SELECT h.checkin_risk_case_id FROM wallet_image_holds h JOIN checkin_risk_cases c ON c.id=h.checkin_risk_case_id WHERE h.batch_id=$1 AND c.status='cleared' FOR UPDATE OF c`, cmd.BatchID).Scan(&caseID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, true, err
		}
		if err == nil && caseID.Valid {
			forfeited := math.Min(refund.Checkin, math.Max(after.Checkin, 0))
			after, err = applyWalletDelta(ctx, tx, cmd.UserID, service.WalletBalance{Checkin: -forfeited}, "checkin_clear_refund", cmd.BatchID, cmd.APIKeyID, nil, fmt.Sprintf("签到违规处理 %d：清退未消费冻结福利", caseID.Int64))
			if err != nil {
				return nil, true, err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE checkin_risk_cases SET cleared_amount=cleared_amount+$1 WHERE id=$2`, forfeited, caseID.Int64); err != nil {
				return nil, true, err
			}
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE wallet_image_holds SET settled=TRUE WHERE batch_id=$1 AND user_id=$2`, cmd.BatchID, cmd.UserID)
	if err != nil {
		return nil, true, err
	}
	var frozen float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET frozen_balance=frozen_balance-$1
		WHERE id=$2 AND frozen_balance >= $1 RETURNING frozen_balance`, cmd.HoldAmount, cmd.UserID).Scan(&frozen)
	return &service.BatchImageBalanceHoldResult{NewBalance: &after.Paid, FrozenBalance: &frozen}, true, err
}
