package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWalletSplit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		wallet WalletBalance
		cost   float64
		want   WalletBalance
	}{
		{"third", WalletBalance{Paid: 20, Checkin: 5}, 9, WalletBalance{Paid: 6, Checkin: 3}},
		{"partial", WalletBalance{Paid: 20, Checkin: 1}, 9, WalletBalance{Paid: 8, Checkin: 1}},
		{"debt", WalletBalance{Paid: 20, Checkin: -5}, 9, WalletBalance{Paid: 9}},
		{"sources", WalletBalance{Paid: 20, Checkin: 1, Liyu: 1, Gift: 4}, 9, WalletBalance{Paid: 6, Checkin: 1, Liyu: 1, Gift: 1}},
		{"exclude negative source", WalletBalance{Paid: 20, Checkin: -5, Gift: 5}, 9, WalletBalance{Paid: 6, Gift: 3}},
		{"no paid", WalletBalance{Checkin: 20}, 9, WalletBalance{Paid: 9}},
		{"round down third", WalletBalance{Paid: 20, Gift: 5}, .00000002, WalletBalance{Paid: .00000002}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.wallet.Split(tc.cost)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.InDelta(t, tc.cost, got.Total(), 1e-10)
			require.LessOrEqual(t, got.PositiveWelfare(), tc.cost/3)
		})
	}
}

func TestWalletAvailableAndInvalidCost(t *testing.T) {
	require.Equal(t, .00000001, (WalletBalance{Paid: .00000001, Checkin: 1}).Available())
	require.Equal(t, -.5, (WalletBalance{Paid: -.5, Checkin: 1}).Available())
	require.Equal(t, 9., (WalletBalance{Paid: 6, Checkin: 30}).Available())
	require.Zero(t, (WalletBalance{Checkin: 30}).Available())
	require.Equal(t, 6., (WalletBalance{Paid: 6, Checkin: -30}).Available())
	for _, amount := range []float64{-1, math.NaN(), math.Inf(1)} {
		_, err := (WalletBalance{Paid: 10}).Split(amount)
		require.Error(t, err)
	}
}
