package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildAccountStabilityExcellent(t *testing.T) {
	stats := &AccountStabilityStats{
		SuccessCount:  200,
		ErrorCount:    1,
		AvgDurationMs: stabilityIntPtr(900),
	}
	account := &Account{Status: StatusActive, Schedulable: true}

	stability := BuildAccountStability(account, stats, 3)

	require.Equal(t, "excellent", stability.Level)
	require.Equal(t, "优秀", stability.Label)
	require.InDelta(t, 200.0/201.0, *stability.SuccessRate, 0.000001)
	require.Equal(t, int64(201), stability.TotalRequests)
}

func TestBuildAccountStabilityHealthy(t *testing.T) {
	stats := &AccountStabilityStats{
		SuccessCount:  95,
		ErrorCount:    5,
		AvgDurationMs: stabilityIntPtr(3200),
	}
	account := &Account{Status: StatusActive, Schedulable: true}

	stability := BuildAccountStability(account, stats, 3)

	require.Equal(t, "healthy", stability.Level)
	require.Equal(t, "健康", stability.Label)
}

func TestBuildAccountStabilityNormal(t *testing.T) {
	stats := &AccountStabilityStats{
		SuccessCount: 80,
		ErrorCount:   20,
	}
	account := &Account{Status: StatusActive, Schedulable: true}

	stability := BuildAccountStability(account, stats, 3)

	require.Equal(t, "normal", stability.Level)
	require.Equal(t, "一般", stability.Label)
}

func TestBuildAccountStabilityDown(t *testing.T) {
	stats := &AccountStabilityStats{
		SuccessCount: 79,
		ErrorCount:   21,
	}
	account := &Account{Status: StatusActive, Schedulable: true}

	stability := BuildAccountStability(account, stats, 3)

	require.Equal(t, "down", stability.Level)
	require.Equal(t, "不通", stability.Label)
}

func TestBuildAccountStabilityUnknownWhenNoRecentData(t *testing.T) {
	account := &Account{Status: StatusActive, Schedulable: true}

	stability := BuildAccountStability(account, &AccountStabilityStats{}, 3)

	require.Equal(t, "unknown", stability.Level)
	require.Equal(t, "无数据", stability.Label)
	require.Nil(t, stability.SuccessRate)
	require.Equal(t, int64(0), stability.TotalRequests)
}

func TestBuildAccountStabilityDownWhenAccountUnschedulable(t *testing.T) {
	until := time.Now().Add(time.Hour)
	account := &Account{
		Status:                 StatusActive,
		Schedulable:            true,
		TempUnschedulableUntil: &until,
	}
	stats := &AccountStabilityStats{SuccessCount: 100}

	stability := BuildAccountStability(account, stats, 3)

	require.Equal(t, "down", stability.Level)
	require.Equal(t, "不通", stability.Label)
	require.Contains(t, stability.Reason, "不可调度")
}

func stabilityIntPtr(v int) *int {
	return &v
}
