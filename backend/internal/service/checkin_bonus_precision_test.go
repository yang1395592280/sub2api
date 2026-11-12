package service

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckinBonusDeltaPrecision(t *testing.T) {
	for _, tc := range []struct {
		name, outcome string
		base, want    float64
	}{
		{"normal win", CheckinBonusStatusWin, 1, 1},
		{"normal loss", CheckinBonusStatusLose, 1, -1.5},
		{"smallest win", CheckinBonusStatusWin, 0.00000001, 0.00000001},
		{"half unit loss", CheckinBonusStatusLose, 0.00000001, -0.00000002},
		{"odd units loss", CheckinBonusStatusLose, 0.00000003, -0.00000005},
		{"fractional loss", CheckinBonusStatusLose, 1.00000001, -1.50000002},
		{"zero loss", CheckinBonusStatusLose, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			delta, err := CheckinBonusDelta(tc.base, tc.base, tc.outcome)
			require.NoError(t, err)
			require.Equal(t, tc.want, delta)
			repo := &checkinRepoStub{getByDateRecord: &CheckinRecord{BaseRewardAmount: tc.base, RewardAmount: tc.base, BonusStatus: CheckinBonusStatusNone}}
			rate := "0"
			if tc.outcome == CheckinBonusStatusWin {
				rate = "100"
			}
			svc := NewCheckinService(repo, newCheckinSettingsWithBonus(true, "0", "2", true, rate), nil, nil)
			_, err = svc.PlayLuckyBonus(context.Background(), 42, "Asia/Shanghai")
			require.NoError(t, err)
			require.Equal(t, tc.want, repo.lastApplyBonusDelta)
		})
	}
}

func TestCheckinBonusDeltaRejectsInvalidInput(t *testing.T) {
	for _, amount := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := CheckinBonusDelta(amount, 1, CheckinBonusStatusWin)
		require.Error(t, err)
		_, err = CheckinBonusDelta(1, amount, CheckinBonusStatusWin)
		require.Error(t, err)
	}
	for _, base := range []float64{-1, 10001} {
		_, err := CheckinBonusDelta(base, base, CheckinBonusStatusWin)
		require.Error(t, err)
	}
	_, err := CheckinBonusDelta(1, 1, "none")
	require.Error(t, err)
}
