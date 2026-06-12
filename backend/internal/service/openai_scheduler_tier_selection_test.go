package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerBuildSelectionOrder_PrimaryBeforeStandby(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{
		stats: newOpenAIAccountRuntimeStats(),
		healthSettings: OpenAISchedulerHealthSettings{
			HealthRankingEnabled: true,
		},
	}
	primary := &Account{ID: 101, Priority: 9}
	standby := &Account{ID: 102, Priority: 1}
	scheduler.seedHealthForTest(primary.ID, openAIAccountHealthRuntime{
		successEWMA:         1,
		consecutiveSuccess:  5,
		lastSelectedUnixSec: 100,
	})
	scheduler.seedHealthForTest(standby.ID, openAIAccountHealthRuntime{
		successEWMA: 0.7,
		errorEWMA:   0.3,
	})

	plan := openAIAccountLoadPlan{
		candidates: []openAIAccountCandidateScore{
			{
				account:  standby,
				loadInfo: &AccountLoadInfo{AccountID: standby.ID, LoadRate: 0, WaitingCount: 0},
				score:    100,
			},
			{
				account:  primary,
				loadInfo: &AccountLoadInfo{AccountID: primary.ID, LoadRate: 100, WaitingCount: 10},
				score:    1,
			},
		},
		topK: 2,
	}

	order := scheduler.buildOpenAISelectionOrder(OpenAIAccountScheduleRequest{SessionHash: "primary-before-standby"}, plan)

	require.Len(t, order, 2)
	require.Equal(t, primary.ID, order[0].account.ID)
	require.Equal(t, standby.ID, order[1].account.ID)
}

func TestOpenAISchedulerBuildSelectionOrder_DegradedLast(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{
		stats: newOpenAIAccountRuntimeStats(),
		healthSettings: OpenAISchedulerHealthSettings{
			HealthRankingEnabled:        true,
			ConsecutiveFailureThreshold: 3,
		},
	}
	healthy := &Account{ID: 201, Priority: 5}
	degraded := &Account{ID: 202, Priority: 1}
	scheduler.seedHealthForTest(healthy.ID, openAIAccountHealthRuntime{
		successEWMA: 0.8,
	})
	scheduler.seedHealthForTest(degraded.ID, openAIAccountHealthRuntime{
		successEWMA:         0.1,
		errorEWMA:           0.9,
		consecutiveFailures: 3,
		lastDegradeReason:   OpenAISchedulerDegradeTimeout,
	})

	plan := openAIAccountLoadPlan{
		candidates: []openAIAccountCandidateScore{
			{
				account:  degraded,
				loadInfo: &AccountLoadInfo{AccountID: degraded.ID, LoadRate: 0, WaitingCount: 0},
				score:    100,
			},
			{
				account:  healthy,
				loadInfo: &AccountLoadInfo{AccountID: healthy.ID, LoadRate: 100, WaitingCount: 10},
				score:    1,
			},
		},
		topK: 2,
	}

	order := scheduler.buildOpenAISelectionOrder(OpenAIAccountScheduleRequest{SessionHash: "degraded-last"}, plan)

	require.Len(t, order, 2)
	require.Equal(t, healthy.ID, order[0].account.ID)
	require.Equal(t, degraded.ID, order[1].account.ID)
}
