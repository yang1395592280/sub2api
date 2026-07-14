package service

import (
	"context"
	"log/slog"
	"sort"
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

type OpenAIBalancedSettings struct {
	Mode                   string
	ShadowMode             bool
	TopK                   int
	ExplorationRate        float64
	LatencyBudgetMS        float64
	SessionEscapeMinGapMS  float64
	SessionEscapeRatio     float64
	SessionEscapeErrorRate float64
}

type OpenAIBalancedCandidate struct {
	AccountID           int64
	HealthKey           OpenAISchedulerHealthKey
	PredictedTTFTMS     float64
	State               string
	ErrorRate           float64
	RateLimitedRate     float64
	ServerErrorRate     float64
	WaitingCount        int
	LoadRate            int
	GroupPriority       int
	Price               float64
	QuotaHeadroom       float64
	LegacyOrderPosition int
	SelectionTier       int
}

type OpenAIBalancedSelectionInput struct {
	PreviousResponseAccountID int64
	SessionAccountID          int64
	Candidates                []OpenAIBalancedCandidate
	LegacyOrderedAccountIDs   []int64
	Settings                  OpenAIBalancedSettings
	RandomSeed                uint64
	Now                       time.Time
	HealthSnapshots           map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot
	HealthLoadAttempted       bool
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
}

type OpenAIBalancedScheduler struct {
	repo OpenAISchedulerHealthRepository
}

func NewOpenAIBalancedScheduler(repo OpenAISchedulerHealthRepository) *OpenAIBalancedScheduler {
	return &OpenAIBalancedScheduler{repo: repo}
}

func DefaultOpenAIBalancedSettings() OpenAIBalancedSettings {
	return OpenAIBalancedSettings{
		Mode:                   OpenAIAutoSchedulerModeBalanced,
		TopK:                   openAIBalancedDefaultTopK,
		ExplorationRate:        openAIBalancedDefaultExplorationRate,
		LatencyBudgetMS:        openAIBalancedDefaultLatencyBudgetMS,
		SessionEscapeMinGapMS:  openAIBalancedDefaultSessionEscapeGapMS,
		SessionEscapeRatio:     openAIBalancedDefaultSessionEscapeRatio,
		SessionEscapeErrorRate: openAIBalancedDefaultSessionErrorRate,
	}
}

func (s *OpenAIBalancedScheduler) Order(ctx context.Context, input OpenAIBalancedSelectionInput) (OpenAIBalancedSelectionResult, error) {
	settings := normalizeOpenAIBalancedSettings(input.Settings)
	if settings.Mode == OpenAIAutoSchedulerModeLegacy {
		return legacyOpenAIBalancedSelectionResult(input, settings), nil
	}
	if input.HealthLoadAttempted {
		loaded, ok := hydrateOpenAIBalancedHealth(input.Candidates, input.HealthSnapshots, input.Now)
		if !ok {
			return openAIBalancedFallbackResult(input, settings), nil
		}
		input.Candidates = loaded
	} else if s != nil && s.repo != nil {
		states, ok := s.loadOpenAIBalancedHealthSnapshots(ctx, input.Candidates, input.Now)
		if !ok {
			return openAIBalancedFallbackResult(input, settings), nil
		}
		loaded, ok := hydrateOpenAIBalancedHealth(input.Candidates, states, input.Now)
		if !ok {
			return openAIBalancedFallbackResult(input, settings), nil
		}
		input.Candidates = loaded
	}
	candidates := make([]OpenAIBalancedCandidate, 0, len(input.Candidates))
	rejectedAccountIDs := make([]int64, 0)
	for _, candidate := range input.Candidates {
		if candidate.AccountID <= 0 {
			continue
		}
		state := normalizeOpenAIAutoSchedulerState(candidate.State)
		if state == OpenAIAutoSchedulerStateOpen || state == OpenAIAutoSchedulerStateHalfOpen {
			rejectedAccountIDs = append(rejectedAccountIDs, candidate.AccountID)
			continue
		}
		candidate.State = state
		candidates = append(candidates, candidate)
	}

	result := OpenAIBalancedSelectionResult{
		CandidateCount:     len(candidates),
		RejectedAccountIDs: rejectedAccountIDs,
	}
	if len(candidates) == 0 {
		if settings.ShadowMode {
			return openAIBalancedShadowResult(input, result), nil
		}
		return result, nil
	}

	bestTTFT := bestOpenAIBalancedTTFT(candidates)
	eligible := make([]OpenAIBalancedCandidate, 0, len(candidates))
	ineligible := make([]OpenAIBalancedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if bestTTFT > 0 && candidate.PredictedTTFTMS > 0 && candidate.PredictedTTFTMS > bestTTFT+settings.LatencyBudgetMS {
			ineligible = append(ineligible, candidate)
			continue
		}
		eligible = append(eligible, candidate)
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		return isOpenAIBalancedCandidateBetter(eligible[i], eligible[j])
	})
	sort.SliceStable(ineligible, func(i, j int) bool {
		return isOpenAIBalancedLatencyTailCandidateBetter(ineligible[i], ineligible[j])
	})
	stickyEscape := ""
	if input.SessionAccountID > 0 {
		stickyEscape = openAIBalancedStickyEscapeReason(input.SessionAccountID, candidates, bestTTFT, settings)
		if stickyEscape == "" {
			eligible, ineligible = promoteOpenAIBalancedCandidate(eligible, ineligible, input.SessionAccountID)
		} else {
			var escaped OpenAIBalancedCandidate
			var found bool
			eligible, escaped, found = removeOpenAIBalancedCandidate(eligible, input.SessionAccountID)
			if !found {
				ineligible, escaped, found = removeOpenAIBalancedCandidate(ineligible, input.SessionAccountID)
			}
			if found {
				ineligible = insertOpenAIBalancedLatencyTailCandidate(ineligible, escaped, candidates)
			}
		}
	}
	if input.PreviousResponseAccountID > 0 {
		eligible, ineligible = promoteOpenAIBalancedCandidate(eligible, ineligible, input.PreviousResponseAccountID)
	}
	latencyEligibleCount := len(eligible)
	ordered := append(append(make([]OpenAIBalancedCandidate, 0, len(candidates)), eligible...), ineligible...)

	topK := settings.TopK
	if topK > latencyEligibleCount {
		topK = latencyEligibleCount
	}
	result.TopK = topK
	result.StickyEscapeReason = stickyEscape
	if topK <= 0 {
		return result, nil
	}
	balancedTop := ordered[:topK]
	strongSticky := input.PreviousResponseAccountID > 0 || (input.SessionAccountID > 0 && stickyEscape == "")
	if settings.ExplorationRate > 0 && len(balancedTop) > 1 {
		rng := newOpenAISelectionRNG(input.RandomSeed)
		if rng.nextFloat64() < settings.ExplorationRate {
			targetIndex := 0
			if strongSticky {
				targetIndex = 1
			}
			explorable := make([]int, 0, len(balancedTop)-targetIndex-1)
			for i := targetIndex + 1; i < len(balancedTop); i++ {
				if balancedTop[i].AccountID != input.SessionAccountID &&
					balancedTop[i].State == OpenAIAutoSchedulerStateRunning &&
					balancedTop[i].WaitingCount == 0 && balancedTop[i].ErrorRate == 0 &&
					balancedTop[i].RateLimitedRate == 0 && balancedTop[i].ServerErrorRate == 0 {
					explorable = append(explorable, i)
				}
			}
			if len(explorable) > 0 {
				idx := explorable[int(rng.nextUint64()%uint64(len(explorable)))]
				candidate := balancedTop[idx]
				copy(balancedTop[targetIndex+1:idx+1], balancedTop[targetIndex:idx])
				balancedTop[targetIndex] = candidate
			}
		} else {
			weightedStart := 0
			if strongSticky {
				weightedStart = 1
			}
			weightedEnd := len(balancedTop)
			if stickyEscape != "" && weightedEnd > weightedStart && balancedTop[weightedEnd-1].AccountID == input.SessionAccountID {
				weightedEnd--
			}
			applyOpenAIBalancedRankWeightedOrder(balancedTop[weightedStart:weightedEnd], &rng)
		}
	}
	result.OrderedAccountIDs = make([]int64, 0, len(ordered))
	for _, candidate := range ordered {
		result.OrderedAccountIDs = append(result.OrderedAccountIDs, candidate.AccountID)
	}
	if settings.ShadowMode {
		result = openAIBalancedShadowResult(input, result)
	}
	return result, nil
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
	balanced.Shadow = true
	balanced.RejectedAccountIDs = nil
	if len(legacyOrder) > 0 {
		balanced.LegacyAccountID = legacyOrder[0]
	}
	if len(balanced.OrderedAccountIDs) > 0 {
		balanced.ShadowAccountID = balanced.OrderedAccountIDs[0]
	}
	legacyTTFT := openAIBalancedCandidateTTFT(input.Candidates, balanced.LegacyAccountID)
	shadowTTFT := openAIBalancedCandidateTTFT(input.Candidates, balanced.ShadowAccountID)
	balanced.PredictedTTFTDifferenceMS = shadowTTFT - legacyTTFT
	switch {
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
			return nil, false
		}
		keys = append(keys, key)
	}
	states, err := s.repo.GetBatch(ctx, keys)
	if err != nil {
		return nil, false
	}
	if now.IsZero() {
		now = time.Now()
	}
	for _, key := range keys {
		snapshot, ok := states[key]
		if !ok || snapshot.ExpiresAt.IsZero() || !now.Before(snapshot.ExpiresAt) {
			return nil, false
		}
	}
	return states, true
}

func hydrateOpenAIBalancedHealth(
	input []OpenAIBalancedCandidate,
	states map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot,
	now time.Time,
) ([]OpenAIBalancedCandidate, bool) {
	if now.IsZero() {
		now = time.Now()
	}
	candidates := append([]OpenAIBalancedCandidate(nil), input...)
	for i := range candidates {
		key := normalizeOpenAISchedulerHealthKey(candidates[i].HealthKey)
		snapshot, ok := states[key]
		if !ok || snapshot.ExpiresAt.IsZero() || !now.Before(snapshot.ExpiresAt) {
			return nil, false
		}
		candidates[i].HealthKey = key
		candidates[i].PredictedTTFTMS = snapshot.PredictedTTFTMS
		candidates[i].State = snapshot.State
		candidates[i].ErrorRate = snapshot.ErrorRate
		candidates[i].RateLimitedRate = snapshot.RateLimitedRate
		candidates[i].ServerErrorRate = snapshot.ServerErrorRate
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
		ModelFamily: resolveOpenAIAccountUpstreamModelForRequest(account, req.RequestedModel, req.RequireCompact),
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
	if settings.Mode != OpenAIAutoSchedulerModeLegacy && settings.Mode != OpenAIAutoSchedulerModeBalanced {
		settings.Mode = OpenAIAutoSchedulerModeBalanced
	}
	if settings.TopK <= 0 {
		settings.TopK = openAIBalancedDefaultTopK
	}
	if settings.LatencyBudgetMS <= 0 {
		settings.LatencyBudgetMS = openAIBalancedDefaultLatencyBudgetMS
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
	return settings
}

func bestOpenAIBalancedTTFT(candidates []OpenAIBalancedCandidate) float64 {
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
