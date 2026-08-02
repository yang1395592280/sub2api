package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
)

const (
	openAIBalancedDefaultTopK               = 3
	openAIBalancedDefaultLatencyBudgetMS    = 1000.0
	openAIBalancedDefaultSessionEscapeGapMS = 1000.0
	openAIBalancedDefaultSessionEscapeRatio = 0.25
	openAIBalancedDefaultSessionErrorRate   = 0.5
	openAIBalancedDefaultExplorationRate    = 0.03
)

const (
	OpenAISchedulerEndpointResponses       = openAISchedulerHealthEndpointResponses
	OpenAISchedulerEndpointChatCompletions = openAISchedulerHealthEndpointChat
	OpenAISchedulerEndpointEmbeddings      = openAISchedulerHealthEndpointEmbeddings
	OpenAISchedulerEndpointImagesGen       = openAISchedulerHealthEndpointImagesGen
	OpenAISchedulerEndpointImagesEdit      = openAISchedulerHealthEndpointImagesEdit
)

const (
	OpenAISchedulerHealthSnapshotFresh   = "fresh"
	OpenAISchedulerHealthSnapshotMissing = "missing"
	OpenAISchedulerHealthSnapshotStale   = "stale"
)

type OpenAIBalancedSettings struct {
	Mode                   string
	ShadowMode             bool
	TopK                   int
	AdaptiveTopKEnabled    bool
	ExplorationRate        float64
	ExplorationBudget      float64
	ExplorationMinInterval time.Duration
	ExplorationMaxSamples  int
	StaleOpenRequiresProbe bool
	RealSampleFreshSeconds int
	LatencyBudgetMS        float64
	SlowThresholdMS        float64
	SessionEscapeMinGapMS  float64
	SessionEscapeRatio     float64
	SessionEscapeErrorRate float64
	Temperature            float64
	MaxAccountShare        float64
	LowConfidenceMaxShare  float64
	Weights                OpenAISchedulerPolicyWeights
}

type OpenAIBalancedCandidate struct {
	AccountID            int64
	HealthKey            OpenAISchedulerHealthKey
	PredictedTTFTMS      float64
	State                string
	ErrorRate            float64
	RateLimitedRate      float64
	ServerErrorRate      float64
	ConsecutiveSlow      int
	ConsecutiveError     int
	WaitingCount         int
	LoadRate             int
	GroupPriority        int
	Price                float64
	QuotaHeadroom        float64
	LegacyOrderPosition  int
	SelectionTier        int
	HealthConfidence     string
	HealthSnapshotStatus string
	LastRealSampleAge    time.Duration
	HasRealSample        bool
	HardRejectedReason   string
}

type OpenAIBalancedSelectionInput struct {
	GroupID                   int64
	AccountSourceGroupID      int64
	AccountSourceType         string
	PoolGroupID               int64
	PoolFallbackReason        string
	PreviousResponseAccountID int64
	SessionAccountID          int64
	Candidates                []OpenAIBalancedCandidate
	LegacyOrderedAccountIDs   []int64
	Settings                  OpenAIBalancedSettings
	RandomSeed                uint64
	Now                       time.Time
	HealthSnapshots           map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot
	HealthLoadAttempted       bool
	HealthLoadSucceeded       bool
}

type OpenAIBalancedSelectionResult struct {
	OrderedAccountIDs         []int64
	RejectedAccountIDs        []int64
	StickyEscapeReason        string
	CandidateCount            int
	TopK                      int
	Shadow                    bool
	LegacyAccountID           int64
	ShadowAccountID           int64
	PredictedTTFTDifferenceMS float64
	ShadowReason              string
	PolicyScores              []OpenAISchedulerPolicyCandidateScore
}

type OpenAIBalancedScheduler struct {
	repo  OpenAISchedulerHealthRepository
	audit *OpenAISchedulerDecisionAuditRecorder
}

func NewOpenAIBalancedScheduler(
	repo OpenAISchedulerHealthRepository,
	audits ...*OpenAISchedulerDecisionAuditRecorder,
) *OpenAIBalancedScheduler {
	scheduler := &OpenAIBalancedScheduler{repo: repo}
	if len(audits) > 0 {
		scheduler.audit = audits[0]
	}
	return scheduler
}

func DefaultOpenAIBalancedSettings() OpenAIBalancedSettings {
	return OpenAIBalancedSettings{
		Mode:                   OpenAIAutoSchedulerModeBalanced,
		TopK:                   openAIBalancedDefaultTopK,
		AdaptiveTopKEnabled:    true,
		ExplorationRate:        openAIBalancedDefaultExplorationRate,
		ExplorationBudget:      DefaultOpenAIAutoSchedulerSettings().ExplorationBudget,
		ExplorationMinInterval: time.Duration(DefaultOpenAIAutoSchedulerSettings().ExplorationMinIntervalSeconds) * time.Second,
		ExplorationMaxSamples:  DefaultOpenAIAutoSchedulerSettings().ExplorationMaxRealSamplesPerHour,
		StaleOpenRequiresProbe: true,
		RealSampleFreshSeconds: DefaultOpenAIAutoSchedulerSettings().RealSampleFreshSeconds,
		LatencyBudgetMS:        openAIBalancedDefaultLatencyBudgetMS,
		SlowThresholdMS:        float64(DefaultOpenAIAutoSchedulerSettings().SlowThresholdMS),
		SessionEscapeMinGapMS:  openAIBalancedDefaultSessionEscapeGapMS,
		SessionEscapeRatio:     openAIBalancedDefaultSessionEscapeRatio,
		SessionEscapeErrorRate: openAIBalancedDefaultSessionErrorRate,
		Temperature:            defaultOpenAISchedulerPolicyTemperature(OpenAIAutoSchedulerModeBalanced),
		MaxAccountShare:        defaultOpenAISchedulerMaxAccountShare(OpenAIAutoSchedulerModeBalanced),
		LowConfidenceMaxShare:  DefaultOpenAIAutoSchedulerSettings().LowConfidenceMaxShare,
		Weights:                defaultOpenAISchedulerPolicyWeights(OpenAIAutoSchedulerModeBalanced),
	}
}

func (s *OpenAIBalancedScheduler) Order(ctx context.Context, input OpenAIBalancedSelectionInput) (OpenAIBalancedSelectionResult, error) {
	settings := normalizeOpenAIBalancedSettings(input.Settings)
	if settings.Mode == OpenAIAutoSchedulerModeLegacy {
		return legacyOpenAIBalancedSelectionResult(input, settings), nil
	}
	if input.HealthLoadAttempted {
		if !input.HealthLoadSucceeded && input.HealthSnapshots == nil {
			result := openAIBalancedFallbackResult(input, settings)
			s.recordShadowDecision(input, result, settings)
			return result, nil
		}
		loaded, ok := hydrateOpenAIBalancedHealth(input.Candidates, input.HealthSnapshots, input.Now, settings.RealSampleFreshSeconds)
		if !ok {
			result := openAIBalancedFallbackResult(input, settings)
			s.recordShadowDecision(input, result, settings)
			return result, nil
		}
		input.Candidates = loaded
	} else if s != nil && s.repo != nil {
		states, ok := s.loadOpenAIBalancedHealthSnapshots(ctx, input.Candidates, input.Now)
		if !ok {
			result := openAIBalancedFallbackResult(input, settings)
			s.recordShadowDecision(input, result, settings)
			return result, nil
		}
		loaded, ok := hydrateOpenAIBalancedHealth(input.Candidates, states, input.Now, settings.RealSampleFreshSeconds)
		if !ok {
			result := openAIBalancedFallbackResult(input, settings)
			s.recordShadowDecision(input, result, settings)
			return result, nil
		}
		input.Candidates = loaded
	}
	candidates := make([]OpenAIBalancedCandidate, 0, len(input.Candidates))
	for _, candidate := range input.Candidates {
		if candidate.AccountID <= 0 {
			continue
		}
		candidate.State = normalizeOpenAIAutoSchedulerState(candidate.State)
		candidates = append(candidates, candidate)
	}
	activeCandidates := make([]OpenAIBalancedCandidate, 0, len(candidates))
	policySettings := openAIAutoSchedulerSettingsFromBalanced(settings)
	eligibilityByAccount := make(map[int64]SchedulerEligibility, len(candidates))
	for _, candidate := range candidates {
		eligibility := EvaluateOpenAISchedulerCandidateEligibility(candidate, policySettings)
		eligibilityByAccount[candidate.AccountID] = eligibility
		if eligibility.Eligible {
			activeCandidates = append(activeCandidates, candidate)
		}
	}
	stickyEscape := ""
	policyCandidates := candidates
	var escapedSticky OpenAIBalancedCandidate
	escapedStickyFound := false
	if input.SessionAccountID > 0 {
		if eligibility, ok := eligibilityByAccount[input.SessionAccountID]; ok && !eligibility.Eligible {
			stickyEscape = eligibility.RejectionCode
			// Keep a session stable when stale health still has a usable TTFT. Queue,
			// error and material TTFT gaps are evaluated by the branch below.
		} else if ok && eligibility.Confidence == "low" && openAIBalancedCandidateTTFT(candidates, input.SessionAccountID) <= 0 {
			stickyEscape = eligibility.RejectionCode
		} else {
			stickyEscape = openAIBalancedStickyEscapeReason(input.SessionAccountID, activeCandidates, bestOpenAIBalancedTTFT(activeCandidates), settings)
		}
		if stickyEscape != "" && eligibilityByAccount[input.SessionAccountID].Eligible {
			policyCandidates, escapedSticky, escapedStickyFound = removeOpenAIBalancedCandidate(policyCandidates, input.SessionAccountID)
		}
	}
	evaluation := EvaluateOpenAISchedulerPolicy(policyCandidates, policySettings, input.RandomSeed)
	ordered := evaluation.OrderedCandidates
	if escapedStickyFound {
		insertAt := evaluation.TopK
		if insertAt > len(ordered) {
			insertAt = len(ordered)
		}
		ordered = append(ordered, OpenAIBalancedCandidate{})
		copy(ordered[insertAt+1:], ordered[insertAt:])
		ordered[insertAt] = escapedSticky
	}
	result := OpenAIBalancedSelectionResult{
		CandidateCount:     len(activeCandidates),
		RejectedAccountIDs: evaluation.RejectedAccountIDs,
		TopK:               evaluation.TopK,
		PolicyScores:       evaluation.Scores,
	}
	if len(ordered) == 0 {
		if settings.ShadowMode {
			result = openAIBalancedShadowResult(input, result)
			s.recordShadowDecision(input, result, settings)
			return result, nil
		}
		if fallback, ok := openAIBalancedSlowOnlyFallback(candidates, input.LegacyOrderedAccountIDs, result, policySettings); ok {
			return fallback, nil
		}
		return result, nil
	}

	if input.SessionAccountID > 0 {
		if stickyEscape == "" {
			ordered = moveOpenAIBalancedCandidateFirst(ordered, input.SessionAccountID)
		}
	}
	if input.PreviousResponseAccountID > 0 {
		ordered = moveOpenAIBalancedCandidateFirst(ordered, input.PreviousResponseAccountID)
	}
	result.StickyEscapeReason = stickyEscape
	result.OrderedAccountIDs = make([]int64, 0, len(ordered))
	for _, candidate := range ordered {
		result.OrderedAccountIDs = append(result.OrderedAccountIDs, candidate.AccountID)
	}
	if settings.ShadowMode {
		result = openAIBalancedShadowResult(input, result)
		s.recordShadowDecision(input, result, settings)
	}
	return result, nil
}

func openAIBalancedSlowOnlyFallback(
	candidates []OpenAIBalancedCandidate,
	legacyOrder []int64,
	result OpenAIBalancedSelectionResult,
	settings OpenAIAutoSchedulerSettings,
) (OpenAIBalancedSelectionResult, bool) {
	if len(candidates) == 0 {
		return result, false
	}
	byID := make(map[int64]OpenAIBalancedCandidate, len(candidates))
	for _, candidate := range candidates {
		eligibility := EvaluateOpenAISchedulerCandidateEligibility(candidate, settings)
		if eligibility.Eligible || candidate.ConsecutiveSlow <= 0 || candidate.ConsecutiveError > 0 ||
			candidate.ErrorRate > 0 || candidate.RateLimitedRate > 0 || candidate.ServerErrorRate > 0 {
			return result, false
		}
		byID[candidate.AccountID] = candidate
	}
	selectedID := int64(0)
	for _, accountID := range legacyOrder {
		if _, ok := byID[accountID]; ok {
			selectedID = accountID
			break
		}
	}
	if selectedID == 0 {
		selectedID = candidates[0].AccountID
	}
	result.OrderedAccountIDs = []int64{selectedID}
	result.RejectedAccountIDs = removeOpenAIBalancedAccountID(result.RejectedAccountIDs, selectedID)
	result.TopK = 1
	result.CandidateCount = 1
	result.PolicyScores = append(result.PolicyScores, OpenAISchedulerPolicyCandidateScore{
		AccountID: selectedID, Eligibility: OpenAISchedulerEligibilityLowConfidence,
		EligibilityReason: "slow_degraded_fallback", TrafficClass: OpenAISchedulerTrafficFallback,
	})
	return result, true
}

func removeOpenAIBalancedAccountID(accountIDs []int64, removed int64) []int64 {
	result := make([]int64, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID != removed {
			result = append(result, accountID)
		}
	}
	return result
}

func (s *OpenAIBalancedScheduler) recordShadowDecision(
	input OpenAIBalancedSelectionInput,
	result OpenAIBalancedSelectionResult,
	settings OpenAIBalancedSettings,
) {
	if s == nil || s.audit == nil || !result.Shadow {
		return
	}
	event := openAISchedulerDecisionAuditFromSettings(settings)
	event.EventType = OpenAISchedulerAuditShadowDecision
	event.GroupID = input.GroupID
	event.EffectiveGroupID = input.GroupID
	event.AccountSourceGroupID = input.AccountSourceGroupID
	event.AccountSourceType = input.AccountSourceType
	event.PoolGroupID = input.PoolGroupID
	event.PoolFallbackReason = input.PoolFallbackReason
	event.AccountID = result.ShadowAccountID
	event.LegacyAccountID = result.LegacyAccountID
	event.Reason = result.ShadowReason
	event.PredictedTTFTDifferenceMS = result.PredictedTTFTDifferenceMS
	event.CandidateCount = result.CandidateCount
	event.TopK = result.TopK
	event.CreatedAt = input.Now

	for _, candidate := range input.Candidates {
		if candidate.AccountID != result.ShadowAccountID {
			continue
		}
		event.ModelFamily = candidate.HealthKey.ModelFamily
		event.Endpoint = candidate.HealthKey.Endpoint
		event.Transport = candidate.HealthKey.Transport
		event.Confidence = candidate.HealthConfidence
		break
	}
	for _, score := range result.PolicyScores {
		if score.AccountID != result.ShadowAccountID {
			continue
		}
		event.Eligibility = score.Eligibility
		event.TrafficClass = score.TrafficClass
		event.TargetShare = score.TargetShare
		break
	}
	s.audit.TryRecord(event)
}

func openAIAutoSchedulerSettingsFromBalanced(settings OpenAIBalancedSettings) OpenAIAutoSchedulerSettings {
	result := DefaultOpenAIAutoSchedulerSettings()
	result.Enabled = true
	result.Mode = settings.Mode
	result.ShadowMode = settings.ShadowMode
	result.TopK = settings.TopK
	result.AdaptiveTopKEnabled = settings.AdaptiveTopKEnabled
	result.ExplorationRate = settings.ExplorationRate
	result.ExplorationBudget = settings.ExplorationBudget
	result.ExplorationMinIntervalSeconds = int(settings.ExplorationMinInterval / time.Second)
	result.ExplorationMaxRealSamplesPerHour = settings.ExplorationMaxSamples
	result.StaleOpenRequiresProbe = settings.StaleOpenRequiresProbe
	result.RealSampleFreshSeconds = settings.RealSampleFreshSeconds
	result.SessionEscapeMinGapMS = int(settings.SessionEscapeMinGapMS)
	result.SessionEscapeRatio = settings.SessionEscapeRatio
	result.LatencyBudgetMS = int(settings.LatencyBudgetMS)
	result.SlowThresholdMS = int(settings.SlowThresholdMS)
	result.Temperature = settings.Temperature
	result.MaxAccountShare = settings.MaxAccountShare
	result.LowConfidenceMaxShare = settings.LowConfidenceMaxShare
	result.Weights = settings.Weights
	return result
}

func openAIBalancedFallbackResult(input OpenAIBalancedSelectionInput, settings OpenAIBalancedSettings) OpenAIBalancedSelectionResult {
	result := legacyOpenAIBalancedSelectionResult(input, settings)
	if !settings.ShadowMode {
		return result
	}
	result.Shadow = true
	if len(result.OrderedAccountIDs) > 0 {
		result.LegacyAccountID = result.OrderedAccountIDs[0]
	}
	result.ShadowReason = "health_unavailable"
	slog.Info("openai_balanced_scheduler_shadow_decision",
		"legacy_account_id", result.LegacyAccountID,
		"shadow_account_id", int64(0),
		"predicted_ttft_difference_ms", float64(0),
		"reason", result.ShadowReason,
	)
	return result
}

func openAIBalancedShadowResult(input OpenAIBalancedSelectionInput, balanced OpenAIBalancedSelectionResult) OpenAIBalancedSelectionResult {
	legacyOrder := append([]int64(nil), input.LegacyOrderedAccountIDs...)
	if len(legacyOrder) == 0 {
		for _, candidate := range input.Candidates {
			if candidate.AccountID > 0 {
				legacyOrder = append(legacyOrder, candidate.AccountID)
			}
		}
	}
	allRejected := len(balanced.OrderedAccountIDs) == 0 && len(balanced.RejectedAccountIDs) > 0
	balanced.Shadow = true
	balanced.RejectedAccountIDs = nil
	if len(legacyOrder) > 0 {
		balanced.LegacyAccountID = legacyOrder[0]
	}
	if len(balanced.OrderedAccountIDs) > 0 {
		balanced.ShadowAccountID = balanced.OrderedAccountIDs[0]
	}
	if !allRejected {
		legacyTTFT := openAIBalancedCandidateTTFT(input.Candidates, balanced.LegacyAccountID)
		shadowTTFT := openAIBalancedCandidateTTFT(input.Candidates, balanced.ShadowAccountID)
		balanced.PredictedTTFTDifferenceMS = shadowTTFT - legacyTTFT
	}
	switch {
	case allRejected:
		balanced.ShadowReason = "all_rejected"
	case balanced.LegacyAccountID == 0 && balanced.ShadowAccountID == 0:
		balanced.ShadowReason = "no_candidate"
	case balanced.LegacyAccountID == balanced.ShadowAccountID:
		balanced.ShadowReason = "same_account"
	case strings.TrimSpace(balanced.StickyEscapeReason) != "":
		balanced.ShadowReason = "sticky_escape_" + balanced.StickyEscapeReason
	default:
		balanced.ShadowReason = "balanced_order_changed"
	}
	slog.Info("openai_balanced_scheduler_shadow_decision",
		"legacy_account_id", balanced.LegacyAccountID,
		"shadow_account_id", balanced.ShadowAccountID,
		"predicted_ttft_difference_ms", balanced.PredictedTTFTDifferenceMS,
		"reason", balanced.ShadowReason,
	)
	balanced.OrderedAccountIDs = legacyOrder
	return balanced
}

func openAIBalancedCandidateTTFT(candidates []OpenAIBalancedCandidate, accountID int64) float64 {
	for _, candidate := range candidates {
		if candidate.AccountID == accountID {
			return candidate.PredictedTTFTMS
		}
	}
	return 0
}

func applyOpenAIBalancedRankWeightedOrder(candidates []OpenAIBalancedCandidate, rng *openAISelectionRNG) {
	if len(candidates) <= 1 || rng == nil {
		return
	}
	pool := append([]OpenAIBalancedCandidate(nil), candidates...)
	weights := make([]int, len(pool))
	for i := range weights {
		weights[i] = len(weights) - i
	}
	for position := range candidates {
		total := 0
		for _, weight := range weights {
			total += weight
		}
		pick := int(rng.nextUint64() % uint64(total))
		selected := 0
		for i, weight := range weights {
			if pick < weight {
				selected = i
				break
			}
			pick -= weight
		}
		candidates[position] = pool[selected]
		pool = append(pool[:selected], pool[selected+1:]...)
		weights = append(weights[:selected], weights[selected+1:]...)
	}
}

func (s *OpenAIBalancedScheduler) loadOpenAIBalancedHealthSnapshots(
	ctx context.Context,
	candidates []OpenAIBalancedCandidate,
	now time.Time,
) (map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, bool) {
	keys := make([]OpenAISchedulerHealthKey, 0, len(candidates))
	for _, candidate := range candidates {
		key := normalizeOpenAISchedulerHealthKey(candidate.HealthKey)
		if !isCompleteOpenAISchedulerHealthKey(key) {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, false
	}
	states, err := s.repo.GetBatch(ctx, keys)
	if err != nil {
		return nil, false
	}
	return states, true
}

func hydrateOpenAIBalancedHealth(
	input []OpenAIBalancedCandidate,
	states map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot,
	now time.Time,
	realSampleFreshSeconds int,
) ([]OpenAIBalancedCandidate, bool) {
	if now.IsZero() {
		now = time.Now()
	}
	candidates := append([]OpenAIBalancedCandidate(nil), input...)
	for i := range candidates {
		key := normalizeOpenAISchedulerHealthKey(candidates[i].HealthKey)
		snapshot, ok := states[key]
		if !ok || snapshot.ExpiresAt.IsZero() || !now.Before(snapshot.ExpiresAt) {
			candidates[i].HealthKey = key
			candidates[i].HealthConfidence = "low"
			if !ok {
				candidates[i].State = OpenAIAutoSchedulerStateRunning
				candidates[i].HealthSnapshotStatus = OpenAISchedulerHealthSnapshotMissing
				continue
			}
			candidates[i].State = normalizeOpenAIAutoSchedulerState(snapshot.State)
			candidates[i].ErrorRate = snapshot.ErrorRate
			candidates[i].RateLimitedRate = snapshot.RateLimitedRate
			candidates[i].ServerErrorRate = snapshot.ServerErrorRate
			candidates[i].ConsecutiveSlow = snapshot.ConsecutiveSlow
			candidates[i].ConsecutiveError = snapshot.ConsecutiveError
			candidates[i].HealthSnapshotStatus = OpenAISchedulerHealthSnapshotStale
			if snapshot.LastRealAt != nil {
				candidates[i].HasRealSample = true
				candidates[i].LastRealSampleAge = now.Sub(*snapshot.LastRealAt)
				if candidates[i].LastRealSampleAge < 0 {
					candidates[i].LastRealSampleAge = 0
				}
			}
			continue
		}
		candidates[i].HealthKey = key
		candidates[i].PredictedTTFTMS = snapshot.PredictedTTFTMS
		candidates[i].State = snapshot.State
		candidates[i].ErrorRate = snapshot.ErrorRate
		candidates[i].RateLimitedRate = snapshot.RateLimitedRate
		candidates[i].ServerErrorRate = snapshot.ServerErrorRate
		candidates[i].ConsecutiveSlow = snapshot.ConsecutiveSlow
		candidates[i].ConsecutiveError = snapshot.ConsecutiveError
		candidates[i].HealthConfidence = classifyOpenAISchedulerHealthConfidence(
			snapshot.State,
			snapshot.ExpiresAt,
			snapshot.RealSampleCount,
			snapshot.ProbeSampleCount,
			snapshot.LastRealAt,
			now,
			realSampleFreshSeconds,
		)
		candidates[i].HealthSnapshotStatus = OpenAISchedulerHealthSnapshotFresh
		if snapshot.LastRealAt != nil {
			candidates[i].HasRealSample = true
			candidates[i].LastRealSampleAge = now.Sub(*snapshot.LastRealAt)
			if candidates[i].LastRealSampleAge < 0 {
				candidates[i].LastRealSampleAge = 0
			}
		}
	}
	return candidates, true
}

func legacyOpenAIBalancedSelectionResult(input OpenAIBalancedSelectionInput, settings OpenAIBalancedSettings) OpenAIBalancedSelectionResult {
	order := append([]int64(nil), input.LegacyOrderedAccountIDs...)
	if len(order) == 0 {
		for _, candidate := range input.Candidates {
			if candidate.AccountID > 0 {
				order = append(order, candidate.AccountID)
			}
		}
	}
	topK := settings.TopK
	if topK > len(order) {
		topK = len(order)
	}
	return OpenAIBalancedSelectionResult{
		OrderedAccountIDs: order,
		CandidateCount:    len(input.Candidates),
		TopK:              topK,
	}
}

func (s *OpenAIGatewayService) openAIBalancedHealthKeyForCandidate(account *Account, req OpenAIAccountScheduleRequest) OpenAISchedulerHealthKey {
	if account == nil {
		return OpenAISchedulerHealthKey{}
	}
	transport := req.RequiredTransport
	if transport == OpenAIUpstreamTransportAny {
		transport = OpenAIUpstreamTransportHTTPSSE
	}
	if transport == OpenAIUpstreamTransportResponsesWebsocketV2Ingress {
		transport = s.openAIBalancedWSIngressTransport(account)
	}
	endpoint := normalizeOpenAISchedulerHealthEndpoint(req.RequiredEndpoint)
	switch transport {
	case OpenAIUpstreamTransportResponsesWebsocket, OpenAIUpstreamTransportResponsesWebsocketV2:
		endpoint = openAISchedulerHealthEndpointResponses
	case OpenAIUpstreamTransportHTTPSSE:
		switch endpoint {
		case openAISchedulerHealthEndpointEmbeddings:
		case openAISchedulerHealthEndpointImagesGen, openAISchedulerHealthEndpointImagesEdit:
			if account.Type == AccountTypeOAuth {
				endpoint = openAISchedulerHealthEndpointResponses
			}
		case openAISchedulerHealthEndpointResponses, openAISchedulerHealthEndpointChat:
			if account.Type == AccountTypeAPIKey && !openai_compat.ShouldUseResponsesAPI(account.Extra) {
				endpoint = openAISchedulerHealthEndpointChat
			} else {
				endpoint = openAISchedulerHealthEndpointResponses
			}
		default:
			endpoint = ""
		}
	default:
		transport = ""
	}
	return normalizeOpenAISchedulerHealthKey(OpenAISchedulerHealthKey{
		AccountID:   account.ID,
		ModelFamily: composeOpenAISchedulerHealthModelFamily(resolveOpenAIAccountUpstreamModelForRequest(account, req.RequestedModel, req.RequireCompact), req.ReasoningEffort),
		Endpoint:    endpoint,
		Transport:   string(transport),
	})
}

func (s *OpenAIGatewayService) openAIBalancedWSIngressTransport(account *Account) OpenAIUpstreamTransport {
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.ModeRouterV2Enabled {
		switch account.ResolveOpenAIResponsesWebSocketV2Mode(s.cfg.Gateway.OpenAIWS.IngressModeDefault) {
		case OpenAIWSIngressModePassthrough:
			return OpenAIUpstreamTransportResponsesWebsocketV2
		case OpenAIWSIngressModeHTTPBridge:
			return OpenAIUpstreamTransportHTTPSSE
		case OpenAIWSIngressModeCtxPool, OpenAIWSIngressModeShared, OpenAIWSIngressModeDedicated:
			return OpenAIUpstreamTransportAny
		default:
			return OpenAIUpstreamTransportAny
		}
	}
	if s == nil {
		return OpenAIUpstreamTransportAny
	}
	return s.getOpenAIWSProtocolResolver().Resolve(account).Transport
}

func normalizeOpenAIBalancedSettings(settings OpenAIBalancedSettings) OpenAIBalancedSettings {
	runtimeSettings := strings.TrimSpace(settings.Mode) != ""
	settings.Mode = strings.ToLower(strings.TrimSpace(settings.Mode))
	if !isSupportedOpenAISchedulerMode(settings.Mode) {
		settings.Mode = OpenAIAutoSchedulerModeBalanced
	}
	if settings.TopK <= 0 {
		settings.TopK = openAIBalancedDefaultTopK
	}
	if settings.ExplorationBudget < 0 || settings.ExplorationBudget > 0.10 {
		settings.ExplorationBudget = DefaultOpenAIAutoSchedulerSettings().ExplorationBudget
	}
	if settings.ExplorationMinInterval <= 0 {
		settings.ExplorationMinInterval = time.Duration(DefaultOpenAIAutoSchedulerSettings().ExplorationMinIntervalSeconds) * time.Second
	}
	if settings.ExplorationMaxSamples <= 0 {
		settings.ExplorationMaxSamples = DefaultOpenAIAutoSchedulerSettings().ExplorationMaxRealSamplesPerHour
	}
	if settings.LatencyBudgetMS <= 0 {
		settings.LatencyBudgetMS = openAIBalancedDefaultLatencyBudgetMS
	}
	if settings.SlowThresholdMS <= 0 {
		settings.SlowThresholdMS = float64(DefaultOpenAIAutoSchedulerSettings().SlowThresholdMS)
	}
	if settings.SessionEscapeMinGapMS < 0 || (!runtimeSettings && settings.SessionEscapeMinGapMS == 0) {
		settings.SessionEscapeMinGapMS = openAIBalancedDefaultSessionEscapeGapMS
	}
	if settings.SessionEscapeRatio < 0 || (!runtimeSettings && settings.SessionEscapeRatio == 0) {
		settings.SessionEscapeRatio = openAIBalancedDefaultSessionEscapeRatio
	}
	if settings.SessionEscapeErrorRate <= 0 || settings.SessionEscapeErrorRate > 1 {
		settings.SessionEscapeErrorRate = openAIBalancedDefaultSessionErrorRate
	}
	if settings.ExplorationRate < 0 || settings.ExplorationRate > 1 {
		settings.ExplorationRate = 0
	}
	if settings.Temperature <= 0 {
		settings.Temperature = defaultOpenAISchedulerPolicyTemperature(settings.Mode)
	}
	if settings.MaxAccountShare <= 0 || settings.MaxAccountShare > 1 {
		settings.MaxAccountShare = defaultOpenAISchedulerMaxAccountShare(settings.Mode)
	}
	if settings.LowConfidenceMaxShare <= 0 || settings.LowConfidenceMaxShare > 1 {
		settings.LowConfidenceMaxShare = DefaultOpenAIAutoSchedulerSettings().LowConfidenceMaxShare
	}
	if settings.RealSampleFreshSeconds <= 0 {
		settings.RealSampleFreshSeconds = DefaultOpenAIAutoSchedulerSettings().RealSampleFreshSeconds
	}
	if openAISchedulerPolicyWeightSum(settings.Weights) <= 0 {
		settings.Weights = defaultOpenAISchedulerPolicyWeights(settings.Mode)
	} else {
		settings.Weights = normalizeOpenAISchedulerPolicyWeights(settings.Weights)
	}
	return settings
}

func bestOpenAIBalancedTTFT(candidates []OpenAIBalancedCandidate) float64 {
	if len(candidates) == 0 {
		return 0
	}
	best := 0.0
	bestTier := candidates[0].SelectionTier
	for _, candidate := range candidates[1:] {
		if candidate.SelectionTier < bestTier {
			bestTier = candidate.SelectionTier
		}
	}
	for _, candidate := range candidates {
		if candidate.SelectionTier != bestTier {
			continue
		}
		if candidate.PredictedTTFTMS > 0 && (best == 0 || candidate.PredictedTTFTMS < best) {
			best = candidate.PredictedTTFTMS
		}
	}
	return best
}

func openAIBalancedStickyEscapeReason(accountID int64, candidates []OpenAIBalancedCandidate, bestTTFT float64, settings OpenAIBalancedSettings) string {
	for _, candidate := range candidates {
		if candidate.AccountID != accountID {
			continue
		}
		if candidate.WaitingCount > 0 {
			return "queue"
		}
		if candidate.RateLimitedRate > 0 {
			return "rate_limited"
		}
		if candidate.ServerErrorRate > 0 {
			return "server_error"
		}
		if candidate.ErrorRate > settings.SessionEscapeErrorRate {
			return "error_rate"
		}
		if candidate.PredictedTTFTMS > 0 && bestTTFT > 0 {
			gap := candidate.PredictedTTFTMS - bestTTFT
			if gap >= settings.SessionEscapeMinGapMS && gap/bestTTFT > settings.SessionEscapeRatio {
				return "ttft"
			}
		}
		return ""
	}
	return ""
}

func isOpenAIBalancedCandidateBetter(left, right OpenAIBalancedCandidate) bool {
	if left.SelectionTier != right.SelectionTier {
		return left.SelectionTier < right.SelectionTier
	}
	if left.PredictedTTFTMS != right.PredictedTTFTMS {
		if left.PredictedTTFTMS <= 0 {
			return false
		}
		if right.PredictedTTFTMS <= 0 {
			return true
		}
		return left.PredictedTTFTMS < right.PredictedTTFTMS
	}
	if left.ErrorRate != right.ErrorRate {
		return left.ErrorRate < right.ErrorRate
	}
	if left.RateLimitedRate != right.RateLimitedRate {
		return left.RateLimitedRate < right.RateLimitedRate
	}
	if left.ServerErrorRate != right.ServerErrorRate {
		return left.ServerErrorRate < right.ServerErrorRate
	}
	if left.WaitingCount != right.WaitingCount {
		return left.WaitingCount < right.WaitingCount
	}
	if left.GroupPriority != right.GroupPriority {
		return left.GroupPriority < right.GroupPriority
	}
	if left.Price != right.Price {
		return left.Price < right.Price
	}
	if left.QuotaHeadroom != right.QuotaHeadroom {
		return left.QuotaHeadroom > right.QuotaHeadroom
	}
	if left.LoadRate != right.LoadRate {
		return left.LoadRate < right.LoadRate
	}
	return left.AccountID < right.AccountID
}

func isOpenAIBalancedLatencyTailCandidateBetter(left, right OpenAIBalancedCandidate) bool {
	return left.LegacyOrderPosition < right.LegacyOrderPosition
}

func moveOpenAIBalancedCandidateFirst(candidates []OpenAIBalancedCandidate, accountID int64) []OpenAIBalancedCandidate {
	for i, candidate := range candidates {
		if candidate.AccountID == accountID {
			copy(candidates[1:i+1], candidates[0:i])
			candidates[0] = candidate
			break
		}
	}
	return candidates
}

func removeOpenAIBalancedCandidate(
	candidates []OpenAIBalancedCandidate,
	accountID int64,
) ([]OpenAIBalancedCandidate, OpenAIBalancedCandidate, bool) {
	for i, candidate := range candidates {
		if candidate.AccountID == accountID {
			return append(candidates[:i], candidates[i+1:]...), candidate, true
		}
	}
	return candidates, OpenAIBalancedCandidate{}, false
}

func insertOpenAIBalancedLatencyTailCandidate(
	tail []OpenAIBalancedCandidate,
	candidate OpenAIBalancedCandidate,
	legacyCandidates []OpenAIBalancedCandidate,
) []OpenAIBalancedCandidate {
	legacyPosition := make(map[int64]int, len(legacyCandidates))
	for i, item := range legacyCandidates {
		legacyPosition[item.AccountID] = i
	}
	insertAt := len(tail)
	for i, item := range tail {
		if candidate.LegacyOrderPosition < item.LegacyOrderPosition ||
			(candidate.LegacyOrderPosition == item.LegacyOrderPosition && legacyPosition[candidate.AccountID] < legacyPosition[item.AccountID]) {
			insertAt = i
			break
		}
	}
	tail = append(tail, OpenAIBalancedCandidate{})
	copy(tail[insertAt+1:], tail[insertAt:])
	tail[insertAt] = candidate
	return tail
}

func promoteOpenAIBalancedCandidate(primary, tail []OpenAIBalancedCandidate, accountID int64) ([]OpenAIBalancedCandidate, []OpenAIBalancedCandidate) {
	for _, candidate := range primary {
		if candidate.AccountID == accountID {
			return moveOpenAIBalancedCandidateFirst(primary, accountID), tail
		}
	}
	for i, candidate := range tail {
		if candidate.AccountID == accountID {
			tail = append(tail[:i], tail[i+1:]...)
			primary = append([]OpenAIBalancedCandidate{candidate}, primary...)
			break
		}
	}
	return primary, tail
}
