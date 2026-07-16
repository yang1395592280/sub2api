package service

import (
	"math"
	"sort"
)

const (
	OpenAISchedulerEligibilityEligible      = "eligible"
	OpenAISchedulerEligibilityLowConfidence = "low_confidence"
	OpenAISchedulerEligibilityLatencyTail   = "latency_tail"
	OpenAISchedulerEligibilityRejected      = "hard_rejected"
)

type OpenAISchedulerPolicyCandidateScore struct {
	AccountID         int64
	Eligibility       string
	EligibilityReason string
	Rank              int
	LatencyScore      float64
	ReliabilityScore  float64
	CostScore         float64
	CapacityScore     float64
	QuotaScore        float64
	PriorityScore     float64
	Utility           float64
	TargetShare       float64
}

type OpenAISchedulerPolicyEvaluation struct {
	OrderedCandidates  []OpenAIBalancedCandidate
	Scores             []OpenAISchedulerPolicyCandidateScore
	RejectedAccountIDs []int64
	TopK               int
}

type openAISchedulerScoredCandidate struct {
	candidate OpenAIBalancedCandidate
	score     OpenAISchedulerPolicyCandidateScore
}

func EvaluateOpenAISchedulerPolicy(
	candidates []OpenAIBalancedCandidate,
	settings OpenAIAutoSchedulerSettings,
	randomSeed uint64,
) OpenAISchedulerPolicyEvaluation {
	settings = normalizeOpenAIAutoSchedulerSettings(settings)
	result := OpenAISchedulerPolicyEvaluation{}
	if len(candidates) == 0 {
		return result
	}

	bestTTFT := 0.0
	minPrice, maxPrice := math.Inf(1), 0.0
	minPriority, maxPriority := candidates[0].GroupPriority, candidates[0].GroupPriority
	minSelectionTier := candidates[0].SelectionTier
	maxWaiting := 1
	for _, candidate := range candidates[1:] {
		if candidate.SelectionTier < minSelectionTier {
			minSelectionTier = candidate.SelectionTier
		}
	}
	for _, candidate := range candidates {
		state := normalizeOpenAIAutoSchedulerState(candidate.State)
		if state != OpenAIAutoSchedulerStateOpen && state != OpenAIAutoSchedulerStateHalfOpen &&
			candidate.SelectionTier == minSelectionTier && candidate.PredictedTTFTMS > 0 &&
			(bestTTFT == 0 || candidate.PredictedTTFTMS < bestTTFT) {
			bestTTFT = candidate.PredictedTTFTMS
		}
		if candidate.Price > 0 {
			if candidate.Price < minPrice {
				minPrice = candidate.Price
			}
			if candidate.Price > maxPrice {
				maxPrice = candidate.Price
			}
		}
		if candidate.GroupPriority < minPriority {
			minPriority = candidate.GroupPriority
		}
		if candidate.GroupPriority > maxPriority {
			maxPriority = candidate.GroupPriority
		}
		if candidate.WaitingCount > maxWaiting {
			maxWaiting = candidate.WaitingCount
		}
	}

	eligible := make([]openAISchedulerScoredCandidate, 0, len(candidates))
	tail := make([]openAISchedulerScoredCandidate, 0)
	rejected := make([]openAISchedulerScoredCandidate, 0)
	for _, candidate := range candidates {
		score := scoreOpenAISchedulerPolicyCandidate(candidate, settings, bestTTFT, minPrice, maxPrice, minPriority, maxPriority, maxWaiting, minSelectionTier)
		item := openAISchedulerScoredCandidate{candidate: candidate, score: score}
		switch score.Eligibility {
		case OpenAISchedulerEligibilityRejected:
			result.RejectedAccountIDs = append(result.RejectedAccountIDs, candidate.AccountID)
			rejected = append(rejected, item)
		case OpenAISchedulerEligibilityLatencyTail:
			tail = append(tail, item)
		default:
			eligible = append(eligible, item)
		}
	}

	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].score.Utility != eligible[j].score.Utility {
			return eligible[i].score.Utility > eligible[j].score.Utility
		}
		return eligible[i].candidate.AccountID < eligible[j].candidate.AccountID
	})
	sort.SliceStable(tail, func(i, j int) bool {
		return tail[i].candidate.LegacyOrderPosition < tail[j].candidate.LegacyOrderPosition
	})

	topK := settings.TopK
	if topK > len(eligible) {
		topK = len(eligible)
	}
	result.TopK = topK
	if topK > 0 {
		shares := openAISchedulerPolicyShares(eligible[:topK], settings)
		for i := 0; i < topK; i++ {
			eligible[i].score.TargetShare = shares[i]
		}
		weighted := weightedOpenAISchedulerPolicyOrder(eligible[:topK], shares, randomSeed)
		copy(eligible[:topK], weighted)
	}

	ordered := append(append(make([]openAISchedulerScoredCandidate, 0, len(eligible)+len(tail)), eligible...), tail...)
	result.OrderedCandidates = make([]OpenAIBalancedCandidate, 0, len(ordered))
	scoresByID := make(map[int64]OpenAISchedulerPolicyCandidateScore, len(ordered))
	for _, item := range ordered {
		result.OrderedCandidates = append(result.OrderedCandidates, item.candidate)
		scoresByID[item.candidate.AccountID] = item.score
	}

	ranked := append([]openAISchedulerScoredCandidate(nil), eligible...)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score.Utility != ranked[j].score.Utility {
			return ranked[i].score.Utility > ranked[j].score.Utility
		}
		return ranked[i].candidate.AccountID < ranked[j].candidate.AccountID
	})
	for i := range ranked {
		score := scoresByID[ranked[i].candidate.AccountID]
		score.Rank = i + 1
		result.Scores = append(result.Scores, score)
	}
	for _, item := range tail {
		result.Scores = append(result.Scores, scoresByID[item.candidate.AccountID])
	}
	for _, item := range rejected {
		result.Scores = append(result.Scores, item.score)
	}
	return result
}

func scoreOpenAISchedulerPolicyCandidate(
	candidate OpenAIBalancedCandidate,
	settings OpenAIAutoSchedulerSettings,
	bestTTFT, minPrice, maxPrice float64,
	minPriority, maxPriority, maxWaiting int,
	minSelectionTier int,
) OpenAISchedulerPolicyCandidateScore {
	score := OpenAISchedulerPolicyCandidateScore{AccountID: candidate.AccountID, Eligibility: OpenAISchedulerEligibilityEligible}
	if candidate.HardRejectedReason != "" {
		score.Eligibility = OpenAISchedulerEligibilityRejected
		score.EligibilityReason = candidate.HardRejectedReason
		return score
	}
	state := normalizeOpenAIAutoSchedulerState(candidate.State)
	if state == OpenAIAutoSchedulerStateOpen || state == OpenAIAutoSchedulerStateHalfOpen {
		score.Eligibility = OpenAISchedulerEligibilityRejected
		score.EligibilityReason = state
		return score
	}
	if candidate.HealthConfidence == "low" || candidate.PredictedTTFTMS <= 0 {
		score.Eligibility = OpenAISchedulerEligibilityLowConfidence
		score.EligibilityReason = "health_unavailable"
	}
	if bestTTFT > 0 && candidate.PredictedTTFTMS > bestTTFT+float64(settings.LatencyBudgetMS) {
		score.Eligibility = OpenAISchedulerEligibilityLatencyTail
		score.EligibilityReason = "latency_budget_exceeded"
	}
	if candidate.SelectionTier > minSelectionTier {
		score.Eligibility = OpenAISchedulerEligibilityLatencyTail
		score.EligibilityReason = "capability_confidence"
	}

	score.LatencyScore = 0.5
	if bestTTFT > 0 && candidate.PredictedTTFTMS > 0 {
		score.LatencyScore = clamp01(bestTTFT / candidate.PredictedTTFTMS)
	}
	score.ReliabilityScore = 1 - clamp01(candidate.ErrorRate+0.5*candidate.RateLimitedRate+0.25*candidate.ServerErrorRate)
	score.CostScore = 0.5
	if candidate.Price > 0 && !math.IsInf(minPrice, 1) && maxPrice > minPrice {
		score.CostScore = 1 - clamp01((candidate.Price-minPrice)/(maxPrice-minPrice))
	}
	loadScore := 1 - clamp01(float64(candidate.LoadRate)/100)
	waitingScore := 1 - clamp01(float64(candidate.WaitingCount)/float64(maxWaiting))
	score.CapacityScore = 0.7*loadScore + 0.3*waitingScore
	score.QuotaScore = clamp01(candidate.QuotaHeadroom)
	score.PriorityScore = 1
	if maxPriority > minPriority {
		score.PriorityScore = 1 - clamp01(float64(candidate.GroupPriority-minPriority)/float64(maxPriority-minPriority))
	}
	weights := settings.Weights
	score.Utility = clamp01(
		weights.Latency*score.LatencyScore +
			weights.Reliability*score.ReliabilityScore +
			weights.Cost*score.CostScore +
			weights.Capacity*score.CapacityScore +
			weights.Quota*score.QuotaScore +
			weights.Priority*score.PriorityScore,
	)
	return score
}

func openAISchedulerPolicyShares(candidates []openAISchedulerScoredCandidate, settings OpenAIAutoSchedulerSettings) []float64 {
	shares := make([]float64, len(candidates))
	if len(candidates) == 0 {
		return shares
	}
	if len(candidates) == 1 {
		shares[0] = 1
		return shares
	}
	maxUtility := candidates[0].score.Utility
	total := 0.0
	for i, candidate := range candidates {
		shares[i] = math.Exp((candidate.score.Utility - maxUtility) / settings.Temperature)
		total += shares[i]
	}
	for i := range shares {
		shares[i] /= total
		shares[i] = (1-settings.ExplorationRate)*shares[i] + settings.ExplorationRate/float64(len(shares))
	}
	caps := make([]float64, len(shares))
	for i, candidate := range candidates {
		caps[i] = settings.MaxAccountShare
		if candidate.score.Eligibility == OpenAISchedulerEligibilityLowConfidence && settings.LowConfidenceMaxShare < caps[i] {
			caps[i] = settings.LowConfidenceMaxShare
		}
	}
	ensureFeasibleOpenAISchedulerShareCaps(candidates, caps)
	return capAndRedistributeOpenAISchedulerShares(shares, caps)
}

// Low-confidence caps are stricter than the normal concentration guard. When
// the configured caps cannot sum to 100%, relax only normal candidates; the
// remaining traffic has no safer alternative.
func ensureFeasibleOpenAISchedulerShareCaps(candidates []openAISchedulerScoredCandidate, caps []float64) {
	totalCap := 0.0
	normal := make([]int, 0, len(candidates))
	for i, candidate := range candidates {
		totalCap += caps[i]
		if candidate.score.Eligibility != OpenAISchedulerEligibilityLowConfidence {
			normal = append(normal, i)
		}
	}
	if totalCap >= 1 || len(normal) == 0 {
		return
	}
	deficit := 1 - totalCap
	for deficit > 1e-9 && len(normal) > 0 {
		perCandidate := deficit / float64(len(normal))
		next := normal[:0]
		for _, index := range normal {
			room := 1 - caps[index]
			increase := math.Min(room, perCandidate)
			caps[index] += increase
			deficit -= increase
			if caps[index] < 1-1e-9 {
				next = append(next, index)
			}
		}
		normal = next
	}
}

func capAndRedistributeOpenAISchedulerShares(shares, caps []float64) []float64 {
	result := append([]float64(nil), shares...)
	for iteration := 0; iteration < len(result)*2; iteration++ {
		excess := 0.0
		availableWeight := 0.0
		for i := range result {
			if result[i] > caps[i] {
				excess += result[i] - caps[i]
				result[i] = caps[i]
			} else if result[i] < caps[i] {
				availableWeight += result[i]
			}
		}
		if excess < 1e-9 || availableWeight <= 0 {
			break
		}
		for i := range result {
			if result[i] < caps[i] {
				result[i] += excess * result[i] / availableWeight
			}
		}
	}
	total := 0.0
	for _, share := range result {
		total += share
	}
	if total > 0 && math.Abs(total-1) > 1e-9 {
		for i := range result {
			result[i] /= total
		}
	}
	return result
}

func weightedOpenAISchedulerPolicyOrder[T any](candidates []T, shares []float64, seed uint64) []T {
	pool := append([]T(nil), candidates...)
	weights := append([]float64(nil), shares...)
	ordered := make([]T, 0, len(pool))
	rng := newOpenAISelectionRNG(seed)
	for len(pool) > 0 {
		total := 0.0
		for _, weight := range weights {
			total += weight
		}
		selected := 0
		if total > 0 {
			pick := rng.nextFloat64() * total
			for i, weight := range weights {
				pick -= weight
				if pick <= 0 {
					selected = i
					break
				}
			}
		}
		ordered = append(ordered, pool[selected])
		pool = append(pool[:selected], pool[selected+1:]...)
		weights = append(weights[:selected], weights[selected+1:]...)
	}
	return ordered
}
