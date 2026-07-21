package service

import (
	"math"
	"sort"
	"strings"
	"time"
)

const (
	OpenAISchedulerEligibilityEligible      = "eligible"
	OpenAISchedulerEligibilityLowConfidence = "low_confidence"
	OpenAISchedulerEligibilityLatencyTail   = "latency_tail"
	OpenAISchedulerEligibilityRejected      = "hard_rejected"

	OpenAISchedulerTrafficNormal      = "normal"
	OpenAISchedulerTrafficExploration = "exploration"
	OpenAISchedulerTrafficFallback    = "fallback"
)

type SchedulerEligibility struct {
	AccountID     int64
	HealthKey     OpenAISchedulerHealthKey
	Eligible      bool
	RecoveryOnly  bool
	Confidence    string
	RejectionCode string
	SnapshotAge   time.Duration
}

type OpenAISchedulerPolicyCandidateScore struct {
	AccountID         int64
	PredictedTTFTMS   float64
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
	TrafficClass      string
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
		eligibility := EvaluateOpenAISchedulerCandidateEligibility(candidate, settings)
		if eligibility.Eligible && eligibility.Confidence == OpenAISchedulerHealthConfidenceHigh &&
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
	if bestTTFT == 0 {
		for _, candidate := range candidates {
			state := normalizeOpenAIAutoSchedulerState(candidate.State)
			if state != OpenAIAutoSchedulerStateOpen && state != OpenAIAutoSchedulerStateHalfOpen &&
				candidate.SelectionTier == minSelectionTier && candidate.PredictedTTFTMS > 0 &&
				(bestTTFT == 0 || candidate.PredictedTTFTMS < bestTTFT) {
				bestTTFT = candidate.PredictedTTFTMS
			}
		}
	}

	normal := make([]openAISchedulerScoredCandidate, 0, len(candidates))
	lowConfidence := make([]openAISchedulerScoredCandidate, 0, len(candidates))
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
		case OpenAISchedulerEligibilityLowConfidence:
			lowConfidence = append(lowConfidence, item)
		default:
			normal = append(normal, item)
		}
	}

	sortOpenAISchedulerCandidatesByUtility(normal)
	sortOpenAISchedulerCandidatesByUtility(lowConfidence)
	sort.SliceStable(tail, func(i, j int) bool {
		return tail[i].candidate.LegacyOrderPosition < tail[j].candidate.LegacyOrderPosition
	})

	topK := effectiveOpenAISchedulerTopK(len(normal), settings)
	result.TopK = topK
	primary := make([]openAISchedulerScoredCandidate, 0, topK+len(lowConfidence))
	primaryShares := make([]float64, 0, cap(primary))
	if topK > 0 {
		exploitation := normal[:topK]
		exploitationShares := openAISchedulerPolicyShares(exploitation, settings)
		exploration := dueOpenAISchedulerExplorationCandidates(lowConfidence, settings)
		explorationBudget := feasibleOpenAISchedulerExplorationBudget(exploration, settings)
		for i := range exploitationShares {
			exploitationShares[i] *= 1 - explorationBudget
			exploitation[i].score.TargetShare = exploitationShares[i]
			exploitation[i].score.TrafficClass = OpenAISchedulerTrafficNormal
		}
		primary = append(primary, exploitation...)
		primaryShares = append(primaryShares, exploitationShares...)
		if len(exploration) > 0 && explorationBudget > 0 {
			share := explorationBudget / float64(len(exploration))
			for i := range exploration {
				exploration[i].score.TargetShare = share
				exploration[i].score.TrafficClass = OpenAISchedulerTrafficExploration
				primary = append(primary, exploration[i])
				primaryShares = append(primaryShares, share)
			}
		}
	} else if len(lowConfidence) > 0 {
		// Availability wins when every candidate is low confidence. This explicit
		// degraded mode is the only case where the exploration budget cannot be
		// enforced because there is no normal pool to receive the remainder.
		result.TopK = effectiveOpenAISchedulerTopK(len(lowConfidence), settings)
		primary = append(primary, lowConfidence[:result.TopK]...)
		primaryShares = openAISchedulerDegradedShares(primary, settings)
		for i := range primary {
			primary[i].score.TargetShare = primaryShares[i]
			primary[i].score.TrafficClass = OpenAISchedulerTrafficFallback
		}
	}
	weightedPrimary := weightedOpenAISchedulerPolicyOrder(primary, primaryShares, randomSeed)
	selected := make(map[int64]struct{}, len(primary))
	for _, item := range primary {
		selected[item.candidate.AccountID] = struct{}{}
	}
	for i := range normal {
		if normal[i].score.TrafficClass == "" {
			normal[i].score.TrafficClass = OpenAISchedulerTrafficFallback
		}
	}
	for i := range lowConfidence {
		if lowConfidence[i].score.TrafficClass == "" {
			lowConfidence[i].score.TrafficClass = OpenAISchedulerTrafficFallback
		}
	}

	ordered := append([]openAISchedulerScoredCandidate(nil), weightedPrimary...)
	for _, pool := range [][]openAISchedulerScoredCandidate{normal, lowConfidence} {
		for _, item := range pool {
			if _, ok := selected[item.candidate.AccountID]; !ok {
				ordered = append(ordered, item)
			}
		}
	}
	ordered = append(ordered, tail...)
	result.OrderedCandidates = make([]OpenAIBalancedCandidate, 0, len(ordered))
	scoresByID := make(map[int64]OpenAISchedulerPolicyCandidateScore, len(ordered))
	for _, item := range primary {
		scoresByID[item.candidate.AccountID] = item.score
	}
	for _, item := range ordered {
		result.OrderedCandidates = append(result.OrderedCandidates, item.candidate)
		if _, ok := scoresByID[item.candidate.AccountID]; !ok {
			scoresByID[item.candidate.AccountID] = item.score
		}
	}

	ranked := append([]openAISchedulerScoredCandidate(nil), normal...)
	ranked = append(ranked, lowConfidence...)
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

func EvaluateOpenAISchedulerCandidateEligibility(
	candidate OpenAIBalancedCandidate,
	settings OpenAIAutoSchedulerSettings,
) SchedulerEligibility {
	result := SchedulerEligibility{
		AccountID:  candidate.AccountID,
		HealthKey:  candidate.HealthKey,
		Eligible:   true,
		Confidence: "high",
	}
	if candidate.HardRejectedReason != "" {
		result.Eligible = false
		result.RejectionCode = candidate.HardRejectedReason
		return result
	}
	state := normalizeOpenAIAutoSchedulerState(candidate.State)
	snapshotStatus := strings.ToLower(strings.TrimSpace(candidate.HealthSnapshotStatus))
	if snapshotStatus == OpenAISchedulerHealthSnapshotStale {
		result.Confidence = "low"
		result.RejectionCode = "health_stale"
		result.SnapshotAge = candidate.LastRealSampleAge
		if settings.StaleOpenRequiresProbe && (state == OpenAIAutoSchedulerStateOpen || state == OpenAIAutoSchedulerStateHalfOpen) {
			result.Eligible = false
			result.RecoveryOnly = true
			result.RejectionCode = "health_stale_recovery_required"
		}
		return result
	}
	if state == OpenAIAutoSchedulerStateOpen {
		result.Eligible = false
		result.RejectionCode = "circuit_open"
		return result
	}
	if state == OpenAIAutoSchedulerStateHalfOpen {
		result.Eligible = false
		result.RecoveryOnly = true
		result.RejectionCode = "half_open_recovery_only"
		return result
	}
	confidence := strings.ToLower(strings.TrimSpace(candidate.HealthConfidence))
	if snapshotStatus == OpenAISchedulerHealthSnapshotMissing ||
		confidence == OpenAISchedulerHealthConfidenceLow ||
		confidence == OpenAISchedulerHealthConfidenceMedium ||
		candidate.PredictedTTFTMS <= 0 {
		result.Confidence = "low"
		if snapshotStatus == OpenAISchedulerHealthSnapshotMissing {
			result.RejectionCode = "health_missing"
		} else {
			result.RejectionCode = "health_unavailable"
		}
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
	score := OpenAISchedulerPolicyCandidateScore{
		AccountID:       candidate.AccountID,
		PredictedTTFTMS: candidate.PredictedTTFTMS,
		Eligibility:     OpenAISchedulerEligibilityEligible,
	}
	eligibility := EvaluateOpenAISchedulerCandidateEligibility(candidate, settings)
	if !eligibility.Eligible {
		score.Eligibility = OpenAISchedulerEligibilityRejected
		score.EligibilityReason = eligibility.RejectionCode
		return score
	}
	if eligibility.Confidence == "low" {
		score.Eligibility = OpenAISchedulerEligibilityLowConfidence
		score.EligibilityReason = eligibility.RejectionCode
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

func sortOpenAISchedulerCandidatesByUtility(candidates []openAISchedulerScoredCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score.Utility != candidates[j].score.Utility {
			return candidates[i].score.Utility > candidates[j].score.Utility
		}
		return candidates[i].candidate.AccountID < candidates[j].candidate.AccountID
	})
}

func effectiveOpenAISchedulerTopK(candidateCount int, settings OpenAIAutoSchedulerSettings) int {
	if candidateCount <= 0 {
		return 0
	}
	topK := settings.TopK
	if settings.AdaptiveTopKEnabled {
		adaptive := 0
		switch {
		case candidateCount <= 2:
			adaptive = candidateCount
		case candidateCount <= 5:
			adaptive = 3
		case candidateCount <= 10:
			adaptive = 4
		case candidateCount <= 20:
			adaptive = 5
		default:
			adaptive = 6
		}
		if adaptive > topK {
			topK = adaptive
		}
	}
	if topK > candidateCount {
		topK = candidateCount
	}
	return topK
}

func dueOpenAISchedulerExplorationCandidates(
	candidates []openAISchedulerScoredCandidate,
	settings OpenAIAutoSchedulerSettings,
) []openAISchedulerScoredCandidate {
	result := make([]openAISchedulerScoredCandidate, 0, len(candidates))
	minimumAge := time.Duration(settings.ExplorationMinIntervalSeconds) * time.Second
	for _, candidate := range candidates {
		if !candidate.candidate.HasRealSample || candidate.candidate.LastRealSampleAge >= minimumAge {
			result = append(result, candidate)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, right := result[i].candidate, result[j].candidate
		if left.HasRealSample != right.HasRealSample {
			return !left.HasRealSample
		}
		if left.LastRealSampleAge != right.LastRealSampleAge {
			return left.LastRealSampleAge > right.LastRealSampleAge
		}
		return left.AccountID < right.AccountID
	})
	return result
}

func feasibleOpenAISchedulerExplorationBudget(candidates []openAISchedulerScoredCandidate, settings OpenAIAutoSchedulerSettings) float64 {
	if len(candidates) == 0 || settings.ExplorationBudget <= 0 {
		return 0
	}
	perAccountCap := settings.LowConfidenceMaxShare
	switch {
	case len(candidates) == 1:
		perAccountCap = math.Min(perAccountCap, 0.15)
	case len(candidates) <= 3:
		perAccountCap = math.Min(perAccountCap, 0.10)
	default:
		perAccountCap = math.Min(perAccountCap, 0.05)
	}
	return math.Min(settings.ExplorationBudget, perAccountCap*float64(len(candidates)))
}

func openAISchedulerDegradedShares(candidates []openAISchedulerScoredCandidate, settings OpenAIAutoSchedulerSettings) []float64 {
	shares := make([]float64, len(candidates))
	if len(candidates) == 0 {
		return shares
	}
	for i := range shares {
		shares[i] = 1 / float64(len(shares))
	}
	return shares
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
