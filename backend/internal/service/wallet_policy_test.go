package service

import (
	"context"
	"math"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestWalletConfiguredRatioAndDebtLimits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		wallet    WalletBalance
		policy    WalletUsagePolicy
		cost      float64
		want      WalletBalance
		available float64
	}{
		{"default", WalletBalance{Paid: 6, Gift: 20}, WalletUsagePolicy{2, 1}, 9, WalletBalance{Paid: 6, Gift: 3}, 9},
		{"quarter", WalletBalance{Paid: 6, Gift: 20}, WalletUsagePolicy{3, 1}, 8, WalletBalance{Paid: 6, Gift: 2}, 8},
		{"disabled", WalletBalance{Paid: 6, Gift: 20}, WalletUsagePolicy{2, 0}, 9, WalletBalance{Paid: 9}, 6},
		{"negative net", WalletBalance{Paid: 6, Checkin: -8, Liyu: 3, Gift: 4}, WalletUsagePolicy{1, 1}, 9, WalletBalance{Paid: 9}, 6},
		{"zero net", WalletBalance{Paid: 6, Checkin: -7, Liyu: 3, Gift: 4}, WalletUsagePolicy{1, 1}, 9, WalletBalance{Paid: 9}, 6},
		{"net cap", WalletBalance{Paid: 6, Checkin: -5, Liyu: 3, Gift: 4}, WalletUsagePolicy{1, 1}, 9, WalletBalance{Paid: 7, Liyu: 2}, 8},
		{"paid matching cap", WalletBalance{Paid: 1, Gift: 20}, WalletUsagePolicy{2, 1}, 9, WalletBalance{Paid: 8.5, Gift: .5}, 1.5},
		{"no paid", WalletBalance{Paid: 0, Gift: 20}, WalletUsagePolicy{1, 1}, 9, WalletBalance{Paid: 9}, 0},
		{"small charge", WalletBalance{Paid: 6, Gift: 20}, WalletUsagePolicy{3, 1}, .00000003, WalletBalance{Paid: .00000003}, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			debit, err := tc.wallet.SplitWithPolicy(tc.cost, tc.policy)
			require.NoError(t, err)
			require.Equal(t, tc.want, debit)
			require.Equal(t, tc.available, tc.wallet.AvailableWithPolicy(tc.policy))
		})
	}
}

func TestWalletSplitInvariants(t *testing.T) {
	// Counterexamples include offsetting debt, micro charges and postpaid overdraft.
	for _, paid := range []float64{-1, 0, .00000001, .1, 6, 100} {
		for _, checkin := range []float64{-7, -1, 0, .00000001, 3, 20} {
			for _, gift := range []float64{0, .00000001, 1, 7, 30} {
				for _, cost := range []float64{0, .00000001, .00000003, .1, 9, 300} {
					for _, p := range []WalletUsagePolicy{{2, 1}, {3, 1}, {1, 1}, {1, 10000}, {10000, 1}, {2, 0}} {
						w := WalletBalance{Paid: paid, Checkin: checkin, Liyu: -.2, Gift: gift}
						d, err := w.SplitWithPolicy(cost, p)
						require.NoError(t, err)
						welfare := decimal.NewFromFloat(d.Checkin).Add(decimal.NewFromFloat(d.Liyu)).Add(decimal.NewFromFloat(d.Gift))
						total := welfare.Add(decimal.NewFromFloat(d.Paid))
						require.True(t, total.Equal(decimal.NewFromFloat(cost)))
						require.True(t, welfare.LessThanOrEqual(decimal.NewFromFloat(w.PositiveWelfare())))
						require.True(t, welfare.Mul(decimal.NewFromInt(p.PaidUnits+p.WelfareUnits)).LessThanOrEqual(decimal.NewFromFloat(cost).Mul(decimal.NewFromInt(p.WelfareUnits))))
						require.True(t, welfare.Mul(decimal.NewFromInt(p.PaidUnits)).LessThanOrEqual(decimal.NewFromFloat(math.Max(0, paid)).Mul(decimal.NewFromInt(p.WelfareUnits))))
						require.LessOrEqual(t, d.Checkin, math.Max(0, checkin))
						require.Zero(t, d.Liyu)
						require.LessOrEqual(t, d.Gift, gift)
					}
				}
			}
		}
	}
}

func TestWalletUsagePolicyValidationAndDefaults(t *testing.T) {
	s := NewCheckinService(nil, &checkinSettingRepoStub{values: map[string]string{}}, nil, nil)
	p, err := s.GetWalletUsagePolicy(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultWalletUsagePolicy(), p)
	for _, raw := range []string{`{}`, `null`, `{"paid_units":0,"welfare_units":1}`, `{"paid_units":2,"welfare_units":-1}`, `{"paid_units":2.5,"welfare_units":1}`, `{"paid_units":10001,"welfare_units":1}`, `{"paid_units":2,"welfare_units":10001}`} {
		_, err := ParseWalletUsagePolicy(raw)
		require.Error(t, err, raw)
	}
	_, err = ParseWalletUsagePolicy(`{"paid_units":3,"welfare_units":0}`)
	require.NoError(t, err)
}

type walletAdmissionUserRepo struct {
	UserRepository
	user *User
}

func (r walletAdmissionUserRepo) GetByID(context.Context, int64) (*User, error) {
	return r.user, nil
}

func TestWalletBillingAdmissionUsesOnlyPaidCredit(t *testing.T) {
	for _, paid := range []float64{-1, 0, 6} {
		for _, policy := range []WalletUsagePolicy{{2, 1}, {1, 10000}, {1, 0}} {
			u := &User{Balance: paid, GiftBalance: 100, WalletUsagePolicy: &policy}
			svc := &BillingCacheService{userRepo: walletAdmissionUserRepo{user: u}}
			balance, err := svc.getUserBalanceFromDB(context.Background(), 42)
			require.NoError(t, err)
			require.Equal(t, paid, balance, "policy changes must not mint admission credit")
		}
	}
}
