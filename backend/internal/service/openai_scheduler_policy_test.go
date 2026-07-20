package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerPolicyWeightsAreNormalizedForEveryMode(t *testing.T) {
	for _, mode := range []string{
		OpenAIAutoSchedulerModeBalanced,
		OpenAIAutoSchedulerModePerformance,
		OpenAIAutoSchedulerModeCost,
		OpenAIAutoSchedulerModeEfficiency,
	} {
		weights := defaultOpenAISchedulerPolicyWeights(mode)
		require.InDelta(t, 1, openAISchedulerPolicyWeightSum(weights), 0.000001, mode)
	}
}

func TestEvaluateOpenAISchedulerPolicyTurnsUtilityGapIntoTargetShareGap(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.TopK = 3
	settings.ExplorationRate = 0
	evaluation := EvaluateOpenAISchedulerPolicy([]OpenAIBalancedCandidate{
		{AccountID: 1, PredictedTTFTMS: 500, State: OpenAIAutoSchedulerStateRunning, Price: 1, QuotaHeadroom: 1},
		{AccountID: 2, PredictedTTFTMS: 1000, State: OpenAIAutoSchedulerStateRunning, Price: 1, QuotaHeadroom: 1},
		{AccountID: 3, PredictedTTFTMS: 1500, State: OpenAIAutoSchedulerStateRunning, Price: 1, QuotaHeadroom: 1},
	}, settings, 42)

	require.Len(t, evaluation.Scores, 3)
	require.Equal(t, int64(1), evaluation.Scores[0].AccountID)
	require.Greater(t, evaluation.Scores[0].TargetShare, evaluation.Scores[1].TargetShare)
	require.Greater(t, evaluation.Scores[1].TargetShare, evaluation.Scores[2].TargetShare)
	require.InDelta(t, 1, policyTargetShareSum(evaluation.Scores), 0.000001)
}

func TestEvaluateOpenAISchedulerPolicyCostModeRewardsCheaperCandidate(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Mode = OpenAIAutoSchedulerModeCost
	settings.Weights = defaultOpenAISchedulerPolicyWeights(settings.Mode)
	settings.TopK = 2
	evaluation := EvaluateOpenAISchedulerPolicy([]OpenAIBalancedCandidate{
		{AccountID: 1, PredictedTTFTMS: 800, State: OpenAIAutoSchedulerStateRunning, Price: 1},
		{AccountID: 2, PredictedTTFTMS: 800, State: OpenAIAutoSchedulerStateRunning, Price: 4},
	}, settings, 1)

	require.Equal(t, int64(1), evaluation.Scores[0].AccountID)
	require.Greater(t, evaluation.Scores[0].CostScore, evaluation.Scores[1].CostScore)
	require.Greater(t, evaluation.Scores[0].TargetShare, evaluation.Scores[1].TargetShare)
}

func TestEvaluateOpenAISchedulerPolicyAppliesShareAndConfidenceCaps(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.TopK = 3
	settings.MaxAccountShare = 0.60
	settings.LowConfidenceMaxShare = 0.10
	evaluation := EvaluateOpenAISchedulerPolicy([]OpenAIBalancedCandidate{
		{AccountID: 1, PredictedTTFTMS: 300, State: OpenAIAutoSchedulerStateRunning, Price: 1},
		{AccountID: 2, PredictedTTFTMS: 900, State: OpenAIAutoSchedulerStateRunning, Price: 2},
		{AccountID: 3, State: OpenAIAutoSchedulerStateRunning, Price: 3, HealthConfidence: "low"},
	}, settings, 9)

	scores := policyScoresByAccount(evaluation.Scores)
	require.LessOrEqual(t, scores[1].TargetShare, 0.60+0.000001)
	require.LessOrEqual(t, scores[3].TargetShare, 0.10+0.000001)
	require.Equal(t, OpenAISchedulerEligibilityLowConfidence, scores[3].Eligibility)
	require.InDelta(t, 1, policyTargetShareSum(evaluation.Scores), 0.000001)
}

func TestEvaluateOpenAISchedulerPolicyKeepsLowConfidenceCapWhenNormalMaxShareIsInfeasible(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.TopK = 2
	settings.MaxAccountShare = 0.70
	settings.LowConfidenceMaxShare = 0.10
	evaluation := EvaluateOpenAISchedulerPolicy([]OpenAIBalancedCandidate{
		{AccountID: 1, PredictedTTFTMS: 800, State: OpenAIAutoSchedulerStateRunning, HealthConfidence: "high"},
		{AccountID: 2, State: OpenAIAutoSchedulerStateRunning, HealthConfidence: "low"},
	}, settings, 7)

	scores := policyScoresByAccount(evaluation.Scores)
	require.LessOrEqual(t, scores[2].TargetShare, 0.10+0.000001)
	require.InDelta(t, 1, policyTargetShareSum(evaluation.Scores), 0.000001)
	require.Greater(t, scores[1].TargetShare, settings.MaxAccountShare)
}

func TestEvaluateOpenAISchedulerPolicyKeepsLatencyAndCapabilityTailsOutOfTargetShare(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.TopK = 3
	settings.LatencyBudgetMS = 1000
	evaluation := EvaluateOpenAISchedulerPolicy([]OpenAIBalancedCandidate{
		{AccountID: 1, PredictedTTFTMS: 500, State: OpenAIAutoSchedulerStateRunning},
		{AccountID: 2, PredictedTTFTMS: 2000, State: OpenAIAutoSchedulerStateRunning},
		{AccountID: 3, PredictedTTFTMS: 400, State: OpenAIAutoSchedulerStateRunning, SelectionTier: 1},
	}, settings, 3)

	scores := policyScoresByAccount(evaluation.Scores)
	require.Equal(t, 1, evaluation.TopK)
	require.Equal(t, OpenAISchedulerEligibilityLatencyTail, scores[2].Eligibility)
	require.Equal(t, OpenAISchedulerEligibilityLatencyTail, scores[3].Eligibility)
	require.Zero(t, scores[2].TargetShare)
	require.Zero(t, scores[3].TargetShare)
}

func TestEvaluateOpenAISchedulerPolicyExploresLowConfidenceCandidatesOutsideTopK(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.TopK = 1
	settings.AdaptiveTopKEnabled = false
	settings.ExplorationBudget = 0.05
	settings.LowConfidenceMaxShare = 0.10
	evaluation := EvaluateOpenAISchedulerPolicy([]OpenAIBalancedCandidate{
		{AccountID: 1, PredictedTTFTMS: 500, State: OpenAIAutoSchedulerStateRunning, HealthConfidence: "high"},
		{AccountID: 2, State: OpenAIAutoSchedulerStateRunning, HealthConfidence: "low"},
		{AccountID: 3, State: OpenAIAutoSchedulerStateRunning, HealthConfidence: "low"},
		{AccountID: 4, State: OpenAIAutoSchedulerStateRunning, HealthConfidence: "low"},
	}, settings, 17)

	scores := policyScoresByAccount(evaluation.Scores)
	require.InDelta(t, 0.95, scores[1].TargetShare, 0.000001)
	for _, accountID := range []int64{2, 3, 4} {
		require.Equal(t, OpenAISchedulerTrafficExploration, scores[accountID].TrafficClass)
		require.InDelta(t, 0.05/3, scores[accountID].TargetShare, 0.000001)
	}
	require.InDelta(t, 1, policyTargetShareSum(evaluation.Scores), 0.000001)
}

func TestEvaluateOpenAISchedulerPolicySkipsRecentLowConfidenceExploration(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.TopK = 1
	settings.AdaptiveTopKEnabled = false
	settings.ExplorationBudget = 0.05
	evaluation := EvaluateOpenAISchedulerPolicy([]OpenAIBalancedCandidate{
		{AccountID: 1, PredictedTTFTMS: 500, State: OpenAIAutoSchedulerStateRunning, HealthConfidence: "high"},
		{AccountID: 2, State: OpenAIAutoSchedulerStateRunning, HealthConfidence: "low", HasRealSample: true, LastRealSampleAge: time.Minute},
	}, settings, 19)

	scores := policyScoresByAccount(evaluation.Scores)
	require.InDelta(t, 1, scores[1].TargetShare, 0.000001)
	require.Zero(t, scores[2].TargetShare)
	require.Equal(t, OpenAISchedulerTrafficFallback, scores[2].TrafficClass)
}

func TestEvaluateOpenAISchedulerPolicyAdaptiveTopKExpandsLargePool(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.TopK = 2
	settings.AdaptiveTopKEnabled = true
	candidates := make([]OpenAIBalancedCandidate, 0, 10)
	for accountID := int64(1); accountID <= 10; accountID++ {
		candidates = append(candidates, OpenAIBalancedCandidate{
			AccountID: accountID, PredictedTTFTMS: 500, State: OpenAIAutoSchedulerStateRunning, HealthConfidence: "high",
		})
	}

	evaluation := EvaluateOpenAISchedulerPolicy(candidates, settings, 23)

	require.Equal(t, 4, evaluation.TopK)
	targeted := 0
	for _, score := range evaluation.Scores {
		if score.TargetShare > 0 {
			targeted++
		}
	}
	require.Equal(t, 4, targeted)
}

func TestOpenAIAutoSchedulerSettingsMapsLegacyCostWeightIntoPolicyWeights(t *testing.T) {
	var settings OpenAIAutoSchedulerSettings
	require.NoError(t, json.Unmarshal([]byte(`{"enabled":true,"mode":"balanced","cost_weight":0.4}`), &settings))

	settings = normalizeOpenAIAutoSchedulerSettings(settings)

	require.Greater(t, settings.Weights.Cost, defaultOpenAISchedulerPolicyWeights(OpenAIAutoSchedulerModeBalanced).Cost)
	require.InDelta(t, 1, openAISchedulerPolicyWeightSum(settings.Weights), 0.000001)
}

func policyScoresByAccount(scores []OpenAISchedulerPolicyCandidateScore) map[int64]OpenAISchedulerPolicyCandidateScore {
	result := make(map[int64]OpenAISchedulerPolicyCandidateScore, len(scores))
	for _, score := range scores {
		result[score.AccountID] = score
	}
	return result
}

func policyTargetShareSum(scores []OpenAISchedulerPolicyCandidateScore) float64 {
	total := 0.0
	for _, score := range scores {
		total += score.TargetShare
	}
	return total
}
