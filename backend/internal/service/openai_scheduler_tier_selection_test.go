package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
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

func TestOpenAISchedulerBuildSelectionOrder_TieredTopKUsesGlobalLimit(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{
		stats: newOpenAIAccountRuntimeStats(),
		healthSettings: OpenAISchedulerHealthSettings{
			HealthRankingEnabled:        true,
			TTFTDegradeMS:               1000,
			ConsecutiveFailureThreshold: 3,
		},
	}
	primary := &Account{ID: 301, Priority: 3}
	standby := &Account{ID: 302, Priority: 2}
	observe := &Account{ID: 303, Priority: 1}
	scheduler.seedHealthForTest(primary.ID, openAIAccountHealthRuntime{
		successEWMA:        1,
		consecutiveSuccess: 5,
	})
	scheduler.seedHealthForTest(standby.ID, openAIAccountHealthRuntime{
		successEWMA: 0.7,
		errorEWMA:   0.3,
	})
	scheduler.seedHealthForTest(observe.ID, openAIAccountHealthRuntime{
		successEWMA: 0.9,
		ttftEWMA:    2000,
	})

	plan := openAIAccountLoadPlan{
		candidates: []openAIAccountCandidateScore{
			{account: observe, loadInfo: &AccountLoadInfo{AccountID: observe.ID}, score: 100},
			{account: standby, loadInfo: &AccountLoadInfo{AccountID: standby.ID}, score: 90},
			{account: primary, loadInfo: &AccountLoadInfo{AccountID: primary.ID}, score: 1},
		},
		topK: 1,
	}

	order := scheduler.buildOpenAISelectionOrder(OpenAIAccountScheduleRequest{SessionHash: "global-top-k"}, plan)

	require.Len(t, order, 1)
	require.Equal(t, primary.ID, order[0].account.ID)
}

func TestOpenAISchedulerBuildSelectionOrder_CompactKeepsSupportedBeforeUnknownAndTiered(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{
		stats: newOpenAIAccountRuntimeStats(),
		healthSettings: OpenAISchedulerHealthSettings{
			HealthRankingEnabled: true,
		},
	}
	supportedPrimary := &Account{ID: 401, Platform: PlatformOpenAI, Extra: map[string]any{"openai_compact_supported": true}, Priority: 9}
	supportedStandby := &Account{ID: 402, Platform: PlatformOpenAI, Extra: map[string]any{"openai_compact_supported": true}, Priority: 1}
	unknownPrimary := &Account{ID: 403, Platform: PlatformOpenAI, Extra: map[string]any{}, Priority: 0}
	scheduler.seedHealthForTest(supportedPrimary.ID, openAIAccountHealthRuntime{
		successEWMA:        1,
		consecutiveSuccess: 5,
	})
	scheduler.seedHealthForTest(supportedStandby.ID, openAIAccountHealthRuntime{
		successEWMA: 0.7,
		errorEWMA:   0.3,
	})
	scheduler.seedHealthForTest(unknownPrimary.ID, openAIAccountHealthRuntime{
		successEWMA:        1,
		consecutiveSuccess: 5,
	})

	plan := openAIAccountLoadPlan{
		allCandidates: []openAIAccountCandidateScore{
			{account: supportedStandby, loadInfo: &AccountLoadInfo{AccountID: supportedStandby.ID}, score: 100},
			{account: unknownPrimary, loadInfo: &AccountLoadInfo{AccountID: unknownPrimary.ID}, score: 95},
			{account: supportedPrimary, loadInfo: &AccountLoadInfo{AccountID: supportedPrimary.ID}, score: 1},
		},
		candidates: []openAIAccountCandidateScore{
			{account: supportedStandby, loadInfo: &AccountLoadInfo{AccountID: supportedStandby.ID}, score: 100},
			{account: unknownPrimary, loadInfo: &AccountLoadInfo{AccountID: unknownPrimary.ID}, score: 95},
			{account: supportedPrimary, loadInfo: &AccountLoadInfo{AccountID: supportedPrimary.ID}, score: 1},
		},
		topK: 3,
	}

	order := scheduler.buildOpenAISelectionOrder(OpenAIAccountScheduleRequest{
		SessionHash:    "compact-supported-first",
		RequireCompact: true,
	}, plan)

	require.Len(t, order, 3)
	require.Equal(t, supportedPrimary.ID, order[0].account.ID)
	require.Equal(t, supportedStandby.ID, order[1].account.ID)
	require.Equal(t, unknownPrimary.ID, order[2].account.ID)
}

func TestOpenAISchedulerBuildOpenAIAccountLoadPlan_FillsHealthAndMixesScore(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{
		stats: newOpenAIAccountRuntimeStats(),
		healthSettings: OpenAISchedulerHealthSettings{
			HealthRankingEnabled:        true,
			ConsecutiveFailureThreshold: 3,
		},
	}
	healthy := &Account{ID: 501, Priority: 1}
	degraded := &Account{ID: 502, Priority: 1}
	scheduler.seedHealthForTest(healthy.ID, openAIAccountHealthRuntime{
		successEWMA:        1,
		consecutiveSuccess: 5,
	})
	scheduler.seedHealthForTest(degraded.ID, openAIAccountHealthRuntime{
		successEWMA:         0.1,
		errorEWMA:           0.9,
		consecutiveFailures: 3,
		lastDegradeReason:   OpenAISchedulerDegradeTimeout,
	})

	plan := scheduler.buildOpenAIAccountLoadPlan(OpenAIAccountScheduleRequest{SessionHash: "load-plan-health"}, []*Account{
		degraded,
		healthy,
	}, map[int64]*AccountLoadInfo{
		healthy.ID:  {AccountID: healthy.ID, LoadRate: 0, WaitingCount: 0},
		degraded.ID: {AccountID: degraded.ID, LoadRate: 0, WaitingCount: 0},
	})

	require.Len(t, plan.allCandidates, 2)
	healthByID := make(map[int64]OpenAIAccountHealthSnapshot, len(plan.allCandidates))
	for _, candidate := range plan.allCandidates {
		healthByID[candidate.account.ID] = candidate.health
	}
	require.Equal(t, healthy.ID, healthByID[healthy.ID].AccountID)
	require.Equal(t, OpenAISchedulerTierPrimary, healthByID[healthy.ID].Tier)
	require.Equal(t, degraded.ID, healthByID[degraded.ID].AccountID)
	require.Equal(t, OpenAISchedulerTierDegraded, healthByID[degraded.ID].Tier)
	require.Len(t, plan.candidates, 1)
	require.Equal(t, healthy.ID, plan.candidates[0].account.ID)
	require.Equal(t, healthy.ID, plan.selectionOrder[0].account.ID)
}

func TestOpenAISchedulerBuildOpenAIAccountLoadPlan_ExcludesDegradedWhenHealthRankingDisabled(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{
		stats:          newOpenAIAccountRuntimeStats(),
		healthSettings: defaultOpenAISchedulerHealthSettings(),
		service:        &OpenAIGatewayService{cfg: &config.Config{}},
	}
	healthy := &Account{ID: 511, Priority: 1}
	degraded := &Account{ID: 512, Priority: 0}
	scheduler.seedHealthForTest(healthy.ID, openAIAccountHealthRuntime{
		successEWMA:        1,
		consecutiveSuccess: 5,
	})
	scheduler.seedHealthForTest(degraded.ID, openAIAccountHealthRuntime{
		successEWMA:         0.1,
		errorEWMA:           0.9,
		consecutiveFailures: 3,
		lastDegradeReason:   OpenAISchedulerDegradeUpstream5xx,
	})

	plan := scheduler.buildOpenAIAccountLoadPlan(OpenAIAccountScheduleRequest{SessionHash: "degraded-filtered-without-ranking"}, []*Account{
		degraded,
		healthy,
	}, map[int64]*AccountLoadInfo{
		healthy.ID:  {AccountID: healthy.ID, LoadRate: 100, WaitingCount: 9},
		degraded.ID: {AccountID: degraded.ID, LoadRate: 0, WaitingCount: 0},
	})

	require.Len(t, plan.allCandidates, 2)
	require.Len(t, plan.candidates, 1)
	require.Equal(t, healthy.ID, plan.candidates[0].account.ID)
	require.Len(t, plan.selectionOrder, 1)
	require.Equal(t, healthy.ID, plan.selectionOrder[0].account.ID)
}

func TestOpenAISchedulerBuildOpenAIAccountLoadPlan_PrefersLowerChannelPriceWhenSpeedComparable(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{
		stats: newOpenAIAccountRuntimeStats(),
		healthSettings: OpenAISchedulerHealthSettings{
			HealthRankingEnabled: true,
		},
		service: &OpenAIGatewayService{},
	}
	cheapPrice := 0.05
	expensivePrice := 0.20
	cheap := &Account{ID: 601, Priority: 1, ChannelPrice: &cheapPrice}
	expensive := &Account{ID: 602, Priority: 1, ChannelPrice: &expensivePrice}
	for _, account := range []*Account{cheap, expensive} {
		scheduler.seedHealthForTest(account.ID, openAIAccountHealthRuntime{
			successEWMA:        1,
			consecutiveSuccess: 5,
			ttftEWMA:           420,
		})
	}

	plan := scheduler.buildOpenAIAccountLoadPlan(OpenAIAccountScheduleRequest{SessionHash: "price-aware"}, []*Account{
		expensive,
		cheap,
	}, map[int64]*AccountLoadInfo{
		cheap.ID:     {AccountID: cheap.ID, LoadRate: 0, WaitingCount: 0},
		expensive.ID: {AccountID: expensive.ID, LoadRate: 0, WaitingCount: 0},
	})

	require.Len(t, plan.candidates, 2)
	scoreByID := make(map[int64]float64, len(plan.candidates))
	for _, candidate := range plan.candidates {
		scoreByID[candidate.account.ID] = candidate.score
	}
	require.Greater(t, scoreByID[cheap.ID], scoreByID[expensive.ID])
	require.Equal(t, cheap.ID, plan.selectionOrder[0].account.ID)
}

func TestOpenAISchedulerBuildOpenAIAccountLoadPlan_BoostsPriceOnlyWhenSpeedGapIsSmall(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 0.7
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 0.8
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 0.5
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Price = 0.2
	cfg.Gateway.OpenAIScheduler.PriceBoostSpeedGapMS = 1000
	cfg.Gateway.OpenAIScheduler.PriceBoostMultiplier = 4
	scheduler := &defaultOpenAIAccountScheduler{
		stats:   newOpenAIAccountRuntimeStats(),
		service: &OpenAIGatewayService{cfg: cfg},
	}
	cheapPrice := 0.05
	expensivePrice := 0.20
	cheapSlightlySlower := &Account{ID: 621, Priority: 1, ChannelPrice: &cheapPrice}
	expensiveFast := &Account{ID: 622, Priority: 1, ChannelPrice: &expensivePrice}
	scheduler.stats.report(cheapSlightlySlower.ID, true, intPtrForTest(650))
	scheduler.stats.report(expensiveFast.ID, true, intPtrForTest(350))

	plan := scheduler.buildOpenAIAccountLoadPlan(OpenAIAccountScheduleRequest{SessionHash: "price-aware-small-speed-gap"}, []*Account{
		cheapSlightlySlower,
		expensiveFast,
	}, map[int64]*AccountLoadInfo{
		cheapSlightlySlower.ID: {AccountID: cheapSlightlySlower.ID, LoadRate: 0, WaitingCount: 0},
		expensiveFast.ID:       {AccountID: expensiveFast.ID, LoadRate: 0, WaitingCount: 0},
	})

	require.Len(t, plan.candidates, 2)
	scoreByID := make(map[int64]float64, len(plan.candidates))
	for _, candidate := range plan.candidates {
		scoreByID[candidate.account.ID] = candidate.score
	}
	require.Greater(t, scoreByID[cheapSlightlySlower.ID], scoreByID[expensiveFast.ID])
}

func TestOpenAISchedulerBuildOpenAIAccountLoadPlan_KeepsFasterAccountWhenSpeedGapIsLarge(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{
		stats:   newOpenAIAccountRuntimeStats(),
		service: &OpenAIGatewayService{},
	}
	cheapPrice := 0.05
	expensivePrice := 0.20
	cheapSlow := &Account{ID: 611, Priority: 1, ChannelPrice: &cheapPrice}
	expensiveFast := &Account{ID: 612, Priority: 1, ChannelPrice: &expensivePrice}
	scheduler.stats.report(cheapSlow.ID, true, intPtrForTest(2200))
	scheduler.stats.report(expensiveFast.ID, true, intPtrForTest(350))

	plan := scheduler.buildOpenAIAccountLoadPlan(OpenAIAccountScheduleRequest{SessionHash: "price-aware-large-speed-gap"}, []*Account{
		cheapSlow,
		expensiveFast,
	}, map[int64]*AccountLoadInfo{
		cheapSlow.ID:     {AccountID: cheapSlow.ID, LoadRate: 0, WaitingCount: 0},
		expensiveFast.ID: {AccountID: expensiveFast.ID, LoadRate: 0, WaitingCount: 0},
	})

	require.Len(t, plan.candidates, 2)
	scoreByID := make(map[int64]float64, len(plan.candidates))
	for _, candidate := range plan.candidates {
		scoreByID[candidate.account.ID] = candidate.score
	}
	require.Greater(t, scoreByID[expensiveFast.ID], scoreByID[cheapSlow.ID])
	require.Equal(t, expensiveFast.ID, plan.selectionOrder[0].account.ID)
}
