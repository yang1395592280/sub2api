package service

import (
	"context"
	"log/slog"
	"math"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AdminWalletUpdater interface {
	UpdateUserWalletBalance(context.Context, int64, float64, float64, int64, string, string) (*User, error)
}

type AdminWalletRefunder interface {
	RefundUserWalletBalance(context.Context, int64, float64, float64, string, int64, string, string) (*User, error)
}

// RefundUserWalletBalance debits only the selected source, atomically with paid credit.
// Activity losses may create welfare debt; administrative refunds must not.
func (s *adminServiceImpl) RefundUserWalletBalance(ctx context.Context, userID int64, paid, welfare float64, source string, actorID int64, notes, operationID string) (*User, error) {
	for _, amount := range []float64{paid, welfare} {
		if amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
			return nil, infraerrors.BadRequest("INVALID_WALLET_REFUND", "退款金额无效")
		}
	}
	paid = QuantizeUsageBillingAmount(paid)
	welfare = QuantizeUsageBillingAmount(welfare)
	if paid == 0 && welfare == 0 {
		return nil, infraerrors.BadRequest("INVALID_WALLET_REFUND", "退款金额至少一项大于0")
	}
	if welfare > 0 && source != "checkin" && source != "liyu" && source != "gift" {
		return nil, infraerrors.BadRequest("INVALID_WELFARE_SOURCE", "请选择要退款的福利来源")
	}
	repo, ok := s.userRepo.(interface {
		RefundWallet(context.Context, int64, float64, float64, string, int64, string, string) error
	})
	if !ok {
		return nil, infraerrors.ServiceUnavailable("WALLET_UNAVAILABLE", "wallet refund unavailable")
	}
	if err := repo.RefundWallet(ctx, userID, paid, welfare, source, actorID, notes, operationID); err != nil {
		return nil, err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			slog.Warn("invalidate balance cache after wallet refund", "user_id", userID, "error", err)
		}
	}
	return s.userRepo.GetByID(ctx, userID)
}

func (s *adminServiceImpl) UpdateUserWalletBalance(ctx context.Context, userID int64, paid, gift float64, actorID int64, notes, operationID string) (*User, error) {
	for _, amount := range []float64{paid, gift} {
		if amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
			return nil, infraerrors.BadRequest("INVALID_WALLET_CREDIT", "invalid credit amount")
		}
	}
	paid = QuantizeUsageBillingAmount(paid)
	gift = QuantizeUsageBillingAmount(gift)
	if paid == 0 && gift == 0 {
		return nil, infraerrors.BadRequest("INVALID_WALLET_CREDIT", "充值或赠送金额至少一项大于0")
	}
	repo, ok := s.userRepo.(interface {
		CreditWallet(context.Context, int64, float64, float64, int64, string, string) (bool, error)
	})
	if !ok {
		return nil, infraerrors.ServiceUnavailable("WALLET_UNAVAILABLE", "wallet credit unavailable")
	}
	applied, err := repo.CreditWallet(ctx, userID, paid, gift, actorID, notes, operationID)
	if err != nil {
		return nil, err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			slog.Warn("invalidate balance cache after wallet credit", "user_id", userID, "error", err)
		}
	}
	if applied {
		s.tryAccrueAffiliateRebateForAdminRecharge(ctx, userID, "add", paid)
	}
	return s.userRepo.GetByID(ctx, userID)
}
