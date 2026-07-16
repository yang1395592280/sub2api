package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/stretchr/testify/require"
)

type balancedSchedulerHealthRepoStub struct {
	states   map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot
	err      error
	getCalls int
	keys     []OpenAISchedulerHealthKey
}

func (r *balancedSchedulerHealthRepoStub) GetBatch(_ context.Context, keys []OpenAISchedulerHealthKey) (map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, error) {
	r.getCalls++
	r.keys = append([]OpenAISchedulerHealthKey(nil), keys...)
	return r.states, r.err
}

func (*balancedSchedulerHealthRepoStub) Upsert(context.Context, OpenAISchedulerHealthSnapshot) error {
	return nil
}

func TestOpenAIBalancedSchedulerEscapesSlowSession(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		SessionAccountID: 1,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 2600, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1200, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 3, SessionEscapeMinGapMS: 1000, SessionEscapeRatio: 0.25},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.OrderedAccountIDs[0])
	require.Equal(t, "ttft", result.StickyEscapeReason)
}

func TestOpenAIBalancedSchedulerPreservesStrongPreviousResponse(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		PreviousResponseAccountID: 1,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 4000, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 500, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 2},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, result.OrderedAccountIDs)
}

func TestOpenAIBalancedSchedulerComparesPriceOnlyInsideLatencyEligiblePool(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 1000, Price: 5, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1000, Price: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 3, PredictedTTFTMS: 4000, Price: 0.01, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 3, LatencyBudgetMS: 1000},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []int64{2, 1, 3}, result.OrderedAccountIDs)
}

func TestOpenAIBalancedSchedulerLatencyTailPreservesLegacyOrderRegardlessOfPrice(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 100, Price: 5, LegacyOrderPosition: 0, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 5000, Price: 10, LegacyOrderPosition: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 3, PredictedTTFTMS: 5000, Price: 1, LegacyOrderPosition: 2, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 3, LatencyBudgetMS: 1000},
	}

	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3}, result.OrderedAccountIDs)
}

func TestOpenAIBalancedSchedulerLatencyTailStrictlyPreservesLegacyOrder(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 100, LegacyOrderPosition: 0, State: OpenAIAutoSchedulerStateRunning},
			{
				AccountID: 2, PredictedTTFTMS: 9000, ErrorRate: 0.8, RateLimitedRate: 0.7,
				ServerErrorRate: 0.6, WaitingCount: 9, LoadRate: 99, GroupPriority: 100,
				QuotaHeadroom: 0.01, LegacyOrderPosition: 1, State: OpenAIAutoSchedulerStateRunning,
			},
			{
				AccountID: 3, PredictedTTFTMS: 3000, ErrorRate: 0.1, RateLimitedRate: 0.1,
				ServerErrorRate: 0.1, WaitingCount: 0, LoadRate: 1, GroupPriority: 1,
				QuotaHeadroom: 0.99, LegacyOrderPosition: 2, State: OpenAIAutoSchedulerStateRunning,
			},
		},
		Settings: OpenAIBalancedSettings{TopK: 3, LatencyBudgetMS: 1000},
	}

	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3}, result.OrderedAccountIDs)
}

func TestOpenAIBalancedSchedulerUsesGroupPriority(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 1000, GroupPriority: 100, Price: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1000, GroupPriority: 1, Price: 1, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 2},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []int64{2, 1}, result.OrderedAccountIDs)
}

func TestOpenAIBalancedSchedulerRanksRateLimitedAndServerErrorCandidatesBehindHealthy(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 1000, RateLimitedRate: 0.2, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1000, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 3, PredictedTTFTMS: 1000, ServerErrorRate: 0.2, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 3},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.OrderedAccountIDs[0])
}

func TestOpenAIBalancedSchedulerEscapesQueuedSession(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		SessionAccountID: 1,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 500, WaitingCount: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1000, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 2},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.OrderedAccountIDs[0])
	require.Equal(t, "queue", result.StickyEscapeReason)
}

func TestOpenAIBalancedSchedulerEscapesSessionAboveErrorRateThreshold(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		SessionAccountID: 1,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 500, ErrorRate: 0.6, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1000, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 2, SessionEscapeErrorRate: 0.5},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.OrderedAccountIDs[0])
	require.Equal(t, "error_rate", result.StickyEscapeReason)
}

func TestOpenAIBalancedSchedulerKeepsCandidatesAfterTopKForSlotFailover(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 1000, GroupPriority: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1000, GroupPriority: 2, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 3, PredictedTTFTMS: 1000, GroupPriority: 3, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 4, PredictedTTFTMS: 1000, GroupPriority: 4, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 2},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 2, result.TopK)
	require.Equal(t, []int64{1, 2, 3, 4}, result.OrderedAccountIDs)
}

func TestOpenAIBalancedSchedulerExcludesOpenAndHalfOpen(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 100, State: OpenAIAutoSchedulerStateOpen},
			{AccountID: 2, PredictedTTFTMS: 200, State: OpenAIAutoSchedulerStateHalfOpen},
			{AccountID: 3, PredictedTTFTMS: 900, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 3},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []int64{3}, result.OrderedAccountIDs)
	require.Equal(t, []int64{1, 2}, result.RejectedAccountIDs)
	require.Equal(t, 1, result.CandidateCount)
}

func TestOpenAIBalancedSchedulerWithoutExplorationIsDeterministic(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 3, PredictedTTFTMS: 1000, GroupPriority: 3, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 1, PredictedTTFTMS: 1000, GroupPriority: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1000, GroupPriority: 2, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 3, ExplorationRate: 0},
	}
	for range 20 {
		result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
		require.NoError(t, err)
		require.Equal(t, []int64{1, 2, 3}, result.OrderedAccountIDs)
	}
}

func TestOpenAIBalancedSchedulerUsesRepeatableSoftmaxAllocation(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		RandomSeed: 81,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 1000, GroupPriority: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1000, GroupPriority: 2, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 3, PredictedTTFTMS: 1000, GroupPriority: 3, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 3, ExplorationRate: 0.03},
	}
	first, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	second, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first.OrderedAccountIDs, second.OrderedAccountIDs)
	require.ElementsMatch(t, []int64{1, 2, 3}, first.OrderedAccountIDs)
	require.Len(t, first.PolicyScores, 3)
}

func TestOpenAIBalancedSchedulerDefaultUsesRepeatableWeightedTopKOrder(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		RandomSeed: 42,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 1000, GroupPriority: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1000, GroupPriority: 2, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 3, PredictedTTFTMS: 1000, GroupPriority: 3, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: DefaultOpenAIBalancedSettings(),
	}
	first, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	second, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first.OrderedAccountIDs, second.OrderedAccountIDs)
	require.ElementsMatch(t, []int64{1, 2, 3}, first.OrderedAccountIDs)
	require.NotEqual(t, []int64{1, 2, 3}, first.OrderedAccountIDs)
}

func TestOpenAIBalancedSchedulerExplorationNeverPromotesLatencyIneligibleCandidate(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		RandomSeed: 81,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 1000, GroupPriority: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1100, GroupPriority: 2, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 3, PredictedTTFTMS: 10000, GroupPriority: 3, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 3, ExplorationRate: 0.03, LatencyBudgetMS: 1000},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 2, result.TopK)
	require.ElementsMatch(t, []int64{1, 2}, result.OrderedAccountIDs[:result.TopK])
	require.Equal(t, int64(3), result.OrderedAccountIDs[2])
}

func TestOpenAIBalancedSchedulerExplorationDoesNotOverrideStrongPreviousResponse(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		PreviousResponseAccountID: 1,
		RandomSeed:                81,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 1000, GroupPriority: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1000, GroupPriority: 2, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 2, ExplorationRate: 0.03},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, result.OrderedAccountIDs)
}

func TestOpenAIBalancedSchedulerEscapedStickyDoesNotPullSlowTailIntoTopK(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		SessionAccountID: 1,
		RandomSeed:       81,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 500, WaitingCount: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 600, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 3, PredictedTTFTMS: 10000, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 3, ExplorationRate: 0.03, LatencyBudgetMS: 1000},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 1, result.TopK)
	require.Equal(t, []int64{2, 1, 3}, result.OrderedAccountIDs)
	require.NotContains(t, result.OrderedAccountIDs[:result.TopK], int64(1))
}

func TestOpenAIBalancedSchedulerCandidateHealthKeyMatchesActualUpstream(t *testing.T) {
	apiKeyResponses := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	apiKeyRawChat := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{
		openai_compat.ExtraKeyResponsesMode:             string(openai_compat.ResponsesSupportModeForceChatCompletions),
		"openai_apikey_responses_websockets_v2_enabled": true,
	}}
	oauth := &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	mapped := &Account{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"model_mapping": map[string]any{"gpt-5": "gpt-5.4"},
	}}

	tests := []struct {
		name      string
		account   *Account
		endpoint  string
		transport OpenAIUpstreamTransport
		want      OpenAISchedulerHealthKey
	}{
		{name: "responses API key", account: apiKeyResponses, endpoint: openAISchedulerHealthEndpointResponses, transport: OpenAIUpstreamTransportHTTPSSE,
			want: OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-5.4", Endpoint: openAISchedulerHealthEndpointResponses, Transport: string(OpenAIUpstreamTransportHTTPSSE)}},
		{name: "responses raw Chat fallback", account: apiKeyRawChat, endpoint: openAISchedulerHealthEndpointResponses, transport: OpenAIUpstreamTransportHTTPSSE,
			want: OpenAISchedulerHealthKey{AccountID: 2, ModelFamily: "gpt-5.4", Endpoint: openAISchedulerHealthEndpointChat, Transport: string(OpenAIUpstreamTransportHTTPSSE)}},
		{name: "Chat OAuth bridge", account: oauth, endpoint: openAISchedulerHealthEndpointChat, transport: OpenAIUpstreamTransportHTTPSSE,
			want: OpenAISchedulerHealthKey{AccountID: 3, ModelFamily: "gpt-5.4", Endpoint: openAISchedulerHealthEndpointResponses, Transport: string(OpenAIUpstreamTransportHTTPSSE)}},
		{name: "WS forces Responses", account: apiKeyRawChat, endpoint: openAISchedulerHealthEndpointChat, transport: OpenAIUpstreamTransportResponsesWebsocketV2Ingress,
			want: OpenAISchedulerHealthKey{AccountID: 2, ModelFamily: "gpt-5.4", Endpoint: openAISchedulerHealthEndpointResponses, Transport: string(OpenAIUpstreamTransportResponsesWebsocketV2)}},
		{name: "embeddings", account: apiKeyRawChat, endpoint: openAISchedulerHealthEndpointEmbeddings, transport: OpenAIUpstreamTransportHTTPSSE,
			want: OpenAISchedulerHealthKey{AccountID: 2, ModelFamily: "text-embedding-3-small", Endpoint: openAISchedulerHealthEndpointEmbeddings, Transport: string(OpenAIUpstreamTransportHTTPSSE)}},
		{name: "OAuth image generation bridge", account: oauth, endpoint: openAISchedulerHealthEndpointImagesGen, transport: OpenAIUpstreamTransportHTTPSSE,
			want: OpenAISchedulerHealthKey{AccountID: 3, ModelFamily: "gpt-image-1", Endpoint: openAISchedulerHealthEndpointResponses, Transport: string(OpenAIUpstreamTransportHTTPSSE)}},
		{name: "API key image generation", account: apiKeyResponses, endpoint: openAISchedulerHealthEndpointImagesGen, transport: OpenAIUpstreamTransportHTTPSSE,
			want: OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-image-1", Endpoint: openAISchedulerHealthEndpointImagesGen, Transport: string(OpenAIUpstreamTransportHTTPSSE)}},
		{name: "API key image edits", account: apiKeyResponses, endpoint: openAISchedulerHealthEndpointImagesEdit, transport: OpenAIUpstreamTransportHTTPSSE,
			want: OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-image-1", Endpoint: openAISchedulerHealthEndpointImagesEdit, Transport: string(OpenAIUpstreamTransportHTTPSSE)}},
		{name: "mapped actual upstream model", account: mapped, endpoint: openAISchedulerHealthEndpointResponses, transport: OpenAIUpstreamTransportHTTPSSE,
			want: OpenAISchedulerHealthKey{AccountID: 4, ModelFamily: "gpt-5.4", Endpoint: openAISchedulerHealthEndpointResponses, Transport: string(OpenAIUpstreamTransportHTTPSSE)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestedModel := tt.want.ModelFamily
			if tt.name == "mapped actual upstream model" {
				requestedModel = "gpt-5"
			}
			svc := &OpenAIGatewayService{}
			if tt.transport == OpenAIUpstreamTransportResponsesWebsocketV2Ingress {
				svc.cfg = newSchedulerTestOpenAIWSV2Config()
			}
			req := OpenAIAccountScheduleRequest{RequestedModel: requestedModel, RequiredEndpoint: tt.endpoint, RequiredTransport: tt.transport}
			require.Equal(t, tt.want, svc.openAIBalancedHealthKeyForCandidate(tt.account, req))
		})
	}
}

func TestOpenAIBalancedSchedulerCandidateHealthKeyNormalizesOAuthUpstreamAlias(t *testing.T) {
	oauthHTTP := &Account{ID: 31, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	oauthWS := &Account{ID: 32, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Extra: map[string]any{
		"openai_oauth_responses_websockets_v2_enabled": true,
	}}

	tests := []struct {
		name      string
		service   *OpenAIGatewayService
		account   *Account
		transport OpenAIUpstreamTransport
	}{
		{name: "Responses HTTP", service: &OpenAIGatewayService{}, account: oauthHTTP, transport: OpenAIUpstreamTransportHTTPSSE},
		{name: "Responses WS", service: &OpenAIGatewayService{cfg: newSchedulerTestOpenAIWSV2Config()}, account: oauthWS, transport: OpenAIUpstreamTransportResponsesWebsocketV2Ingress},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := OpenAIAccountScheduleRequest{
				RequestedModel: "gpt-5.1", RequiredEndpoint: OpenAISchedulerEndpointResponses, RequiredTransport: tt.transport,
			}
			key := tt.service.openAIBalancedHealthKeyForCandidate(tt.account, req)
			require.Equal(t, "gpt-5.4", key.ModelFamily)
			require.Equal(t, normalizeOpenAIModelForUpstream(tt.account, tt.account.GetMappedModel(req.RequestedModel)), key.ModelFamily)
		})
	}
}

func TestOpenAIBalancedSchedulerLoadsHealthInOneBatch(t *testing.T) {
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	key1 := OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse"}
	key2 := OpenAISchedulerHealthKey{AccountID: 2, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse"}
	repo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
		key1: {Key: key1, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 2500, ExpiresAt: now.Add(time.Minute)},
		key2: {Key: key2, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500, ExpiresAt: now.Add(time.Minute)},
	}}
	result, err := NewOpenAIBalancedScheduler(repo).Order(context.Background(), OpenAIBalancedSelectionInput{
		Now: now,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, HealthKey: key1},
			{AccountID: 2, HealthKey: key2},
		},
		LegacyOrderedAccountIDs: []int64{1, 2},
		Settings:                OpenAIBalancedSettings{TopK: 2},
	})
	require.NoError(t, err)
	require.Equal(t, 1, repo.getCalls)
	require.ElementsMatch(t, []OpenAISchedulerHealthKey{key1, key2}, repo.keys)
	require.Equal(t, []int64{2, 1}, result.OrderedAccountIDs)
}

func TestOpenAIBalancedSchedulerShadowReturnsFullLegacyOrderAndRecordsComparison(t *testing.T) {
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 2000, Price: 5, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 500, Price: 1, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 3, PredictedTTFTMS: 4000, Price: 1, State: OpenAIAutoSchedulerStateRunning},
		},
		LegacyOrderedAccountIDs: []int64{1, 2, 3},
		Settings: OpenAIBalancedSettings{
			Mode: "balanced", ShadowMode: true, TopK: 2, ExplorationRate: 0,
		},
	})

	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3}, result.OrderedAccountIDs)
	require.True(t, result.Shadow)
	require.Equal(t, int64(1), result.LegacyAccountID)
	require.Equal(t, int64(2), result.ShadowAccountID)
	require.InDelta(t, -1500, result.PredictedTTFTDifferenceMS, 0.001)
	require.Equal(t, "balanced_order_changed", result.ShadowReason)
}

func TestOpenAIBalancedSchedulerShadowReturnsLegacyWhenAllBalancedCandidatesAreCircuitRejected(t *testing.T) {
	previousLogger := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 2000, State: OpenAIAutoSchedulerStateOpen},
			{AccountID: 2, PredictedTTFTMS: 500, State: OpenAIAutoSchedulerStateHalfOpen},
		},
		LegacyOrderedAccountIDs: []int64{1, 2},
		Settings: OpenAIBalancedSettings{
			Mode: OpenAIAutoSchedulerModeBalanced, ShadowMode: true, TopK: 2,
		},
	})

	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, result.OrderedAccountIDs)
	require.Empty(t, result.RejectedAccountIDs)
	require.True(t, result.Shadow)
	require.Zero(t, result.ShadowAccountID)
	require.Zero(t, result.PredictedTTFTDifferenceMS)
	require.Equal(t, "all_rejected", result.ShadowReason)
	require.Contains(t, logs.String(), "legacy_account_id=1")
	require.Contains(t, logs.String(), "shadow_account_id=0")
	require.Contains(t, logs.String(), "predicted_ttft_difference_ms=0")
	require.Contains(t, logs.String(), "reason=all_rejected")
}

func TestOpenAIBalancedSchedulerHonorsExplicitZeroSessionEscapeThresholds(t *testing.T) {
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), OpenAIBalancedSelectionInput{
		SessionAccountID: 1,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 900, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 500, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{
			Mode: OpenAIAutoSchedulerModeBalanced, TopK: 2,
			SessionEscapeMinGapMS: 0, SessionEscapeRatio: 0,
		},
	})

	require.NoError(t, err)
	require.Equal(t, "ttft", result.StickyEscapeReason)
	require.Equal(t, int64(2), result.OrderedAccountIDs[0])
}

func TestOpenAIBalancedSchedulerLiveReturnsBalancedOrder(t *testing.T) {
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 2000, Price: 5, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 500, Price: 1, State: OpenAIAutoSchedulerStateRunning},
		},
		LegacyOrderedAccountIDs: []int64{1, 2},
		Settings: OpenAIBalancedSettings{
			Mode: "balanced", ShadowMode: false, TopK: 2, ExplorationRate: 0,
		},
	})

	require.NoError(t, err)
	require.Equal(t, []int64{2, 1}, result.OrderedAccountIDs)
	require.False(t, result.Shadow)
}

func TestOpenAIBalancedSchedulerLegacyModeSkipsBalancedHealthLoad(t *testing.T) {
	repo := &balancedSchedulerHealthRepoStub{}
	result, err := NewOpenAIBalancedScheduler(repo).Order(context.Background(), OpenAIBalancedSelectionInput{
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, HealthKey: OpenAISchedulerHealthKey{AccountID: 1}},
			{AccountID: 2, HealthKey: OpenAISchedulerHealthKey{AccountID: 2}},
		},
		LegacyOrderedAccountIDs: []int64{1, 2},
		Settings:                OpenAIBalancedSettings{Mode: "legacy", TopK: 2},
	})

	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, result.OrderedAccountIDs)
	require.Zero(t, repo.getCalls)
}

func TestOpenAIBalancedSchedulerHealthFallbackOnlyOnWholeStoreFailure(t *testing.T) {
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	key1 := OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse"}
	key2 := OpenAISchedulerHealthKey{AccountID: 2, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse"}
	tests := []struct {
		name      string
		repo      *balancedSchedulerHealthRepoStub
		keyTwo    OpenAISchedulerHealthKey
		wantOrder []int64
	}{
		{name: "repository error", repo: &balancedSchedulerHealthRepoStub{err: errors.New("health unavailable")}, keyTwo: key2, wantOrder: []int64{2, 1}},
		{name: "missing snapshot", repo: &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
			key1: {Key: key1, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500, ExpiresAt: now.Add(time.Minute)},
		}}, keyTwo: key2, wantOrder: []int64{1, 2}},
		{name: "expired snapshot", repo: &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
			key1: {Key: key1, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500, ExpiresAt: now.Add(time.Minute)},
			key2: {Key: key2, State: OpenAIAutoSchedulerStateRunning, ExpiresAt: now.Add(-time.Second)},
		}}, keyTwo: key2, wantOrder: []int64{1, 2}},
		{name: "incomplete key", repo: &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
			key1: {Key: key1, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500, ExpiresAt: now.Add(time.Minute)},
		}}, keyTwo: OpenAISchedulerHealthKey{AccountID: 2}, wantOrder: []int64{1, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewOpenAIBalancedScheduler(tt.repo).Order(context.Background(), OpenAIBalancedSelectionInput{
				Now: now,
				Candidates: []OpenAIBalancedCandidate{
					{AccountID: 1, HealthKey: key1},
					{AccountID: 2, HealthKey: tt.keyTwo},
				},
				LegacyOrderedAccountIDs: []int64{2, 1},
				Settings:                OpenAIBalancedSettings{TopK: 2},
			})
			require.NoError(t, err)
			require.Equal(t, tt.wantOrder, result.OrderedAccountIDs)
			require.LessOrEqual(t, tt.repo.getCalls, 1)
		})
	}
}

func TestOpenAIBalancedSchedulerShadowUsesPartialHealthWithoutChangingLegacyOrder(t *testing.T) {
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	key1 := OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse"}
	key2 := OpenAISchedulerHealthKey{AccountID: 2, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse"}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), OpenAIBalancedSelectionInput{
		Now: now,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, HealthKey: key1},
			{AccountID: 2, HealthKey: key2},
		},
		LegacyOrderedAccountIDs: []int64{1, 2},
		Settings: OpenAIBalancedSettings{
			Mode: OpenAIAutoSchedulerModeBalanced, ShadowMode: true, TopK: 2,
		},
		HealthLoadAttempted: true,
		HealthSnapshots: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
			key1: {Key: key1, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500, ExpiresAt: now.Add(time.Minute)},
		},
	})

	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, result.OrderedAccountIDs)
	require.True(t, result.Shadow)
	require.Equal(t, int64(1), result.LegacyAccountID)
	require.Equal(t, int64(1), result.ShadowAccountID)
	require.Zero(t, result.PredictedTTFTDifferenceMS)
	require.Equal(t, "same_account", result.ShadowReason)
}
