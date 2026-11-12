package service

import (
	"context"
	"errors"
	"math"

	"github.com/shopspring/decimal"
)

// WalletBalance keeps earned credit separate from promotional credit. Negative
// promotional balances are activity losses, never an additional usage charge.
type WalletBalance struct {
	Paid    float64 `json:"paid"`
	Checkin float64 `json:"checkin"`
	Liyu    float64 `json:"liyu"`
	Gift    float64 `json:"gift"`
}

func (w WalletBalance) Total() float64 {
	return QuantizeUsageBillingAmount(w.Paid + w.Checkin + w.Liyu + w.Gift)
}

func (w WalletBalance) PositiveWelfare() float64 {
	return QuantizeUsageBillingAmount(math.Max(0, w.Checkin) + math.Max(0, w.Liyu) + math.Max(0, w.Gift))
}

func (w WalletBalance) Available() float64 {
	if w.Paid <= 0 {
		return w.Paid
	}
	paid := decimal.NewFromFloat(w.Paid).Truncate(UsageBillingMonetaryScale)
	allowance := paid.Div(decimal.NewFromInt(2)).Truncate(UsageBillingMonetaryScale)
	return paid.Add(decimal.Min(decimal.NewFromFloat(w.PositiveWelfare()), allowance)).InexactFloat64()
}

// Split returns debit amounts, consuming check-in, liyu, then admin gifts.
// Round the one-third allowance down so tiny charges cannot exceed the subsidy
// limit. Paid debt remains possible for already delivered postpaid requests.
func (w WalletBalance) Split(amount float64) (WalletBalance, error) {
	if amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return WalletBalance{}, errors.New("invalid wallet charge")
	}
	cost := decimal.NewFromFloat(amount).Round(UsageBillingMonetaryScale)
	remaining := cost.Div(decimal.NewFromInt(3)).Truncate(UsageBillingMonetaryScale)
	if w.Paid <= 0 {
		remaining = decimal.Zero
	}
	debit := WalletBalance{}
	for _, item := range []struct {
		balance float64
		debit   *float64
	}{
		{w.Checkin, &debit.Checkin}, {w.Liyu, &debit.Liyu}, {w.Gift, &debit.Gift},
	} {
		take := decimal.Min(remaining, decimal.NewFromFloat(math.Max(0, item.balance)))
		*item.debit = take.InexactFloat64()
		remaining = remaining.Sub(take)
	}
	debit.Paid = cost.Sub(decimal.NewFromFloat(debit.Checkin)).Sub(decimal.NewFromFloat(debit.Liyu)).Sub(decimal.NewFromFloat(debit.Gift)).InexactFloat64()
	return debit, nil
}

func (u *User) Wallet() WalletBalance {
	return WalletBalance{Paid: u.Balance, Checkin: u.CheckinBalance, Liyu: u.LiyuBalance, Gift: u.GiftBalance}
}

type WalletUsageBreakdown struct {
	PaidCost    float64 `json:"paid_cost"`
	CheckinCost float64 `json:"checkin_cost"`
	LiyuCost    float64 `json:"liyu_cost"`
	GiftCost    float64 `json:"gift_cost"`
	WelfareCost float64 `json:"welfare_cost"`
}

// deductWalletUsage preserves the request identity for wallet/usage reconciliation.
func deductWalletUsage(ctx context.Context, repo UserRepository, userID int64, amount float64, requestID string, apiKeyID int64) error {
	if requestID == "" {
		return repo.DeductBalance(ctx, userID, amount)
	}
	if wallet, ok := repo.(interface {
		DeductWalletUsage(context.Context, int64, float64, string, int64) error
	}); ok {
		return wallet.DeductWalletUsage(ctx, userID, amount, requestID, apiKeyID)
	}
	return repo.DeductBalance(ctx, userID, amount)
}
