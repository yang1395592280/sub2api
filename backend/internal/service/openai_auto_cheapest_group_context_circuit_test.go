package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type autoCheapestCircuitStub struct {
	calls       int
	keys        []OpenAIAutoCheapestGroupHealthKey
	successKeys []OpenAIAutoCheapestGroupHealthKey
}

func (s *autoCheapestCircuitStub) Allow(context.Context, OpenAIAutoCheapestGroupHealthKey) (bool, error) {
	return true, nil
}
func (s *autoCheapestCircuitStub) RecordFailure(_ context.Context, key OpenAIAutoCheapestGroupHealthKey, _ string) error {
	s.calls++
	s.keys = append(s.keys, key)
	return nil
}
func (s *autoCheapestCircuitStub) RecordSuccess(_ context.Context, key OpenAIAutoCheapestGroupHealthKey) error {
	s.successKeys = append(s.successKeys, key)
	return nil
}

func TestMarkOpenAIAutoCheapestGroupExhausted_RecordsCircuitOncePerRequest(t *testing.T) {
	circuit := &autoCheapestCircuitStub{}
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true, circuit)
	setOpenAIAutoCheapestGroupFailureUserContext(ctx, 7)

	markOpenAIAutoCheapestGroupExhausted(ctx, 42, "no_available_accounts")
	markOpenAIAutoCheapestGroupExhausted(ctx, 42, "no_available_accounts")

	require.Equal(t, 1, circuit.calls)
	require.Equal(t, int64(7), circuit.keys[0].UserID)
	reason, ok := openAIAutoCheapestGroupExhaustionReason(ctx, 42)
	require.True(t, ok)
	require.Equal(t, "no_available_accounts", reason)
}

func TestShouldQuarantineOpenAIAutoCheapestAccount(t *testing.T) {
	status := func(code int) *int { return &code }
	tests := []struct {
		name    string
		input   OpenAIAutoSchedulerRecordInput
		quarant bool
	}{
		{name: "rate limited", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventRateLimited, StatusCode: status(http.StatusTooManyRequests)}, quarant: true},
		{name: "server error", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, StatusCode: status(http.StatusBadGateway)}, quarant: true},
		{name: "stream disconnect", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, Message: "upstream stream disconnected"}, quarant: true},
		{name: "network error", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, Message: "network error"}, quarant: true},
		{name: "unexpected eof", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, Message: "read: unexpected EOF"}, quarant: true},
		{name: "http 200 stream reset", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, StatusCode: status(http.StatusOK), Message: "stream usage incomplete: stream error: stream ID 1; INTERNAL_ERROR"}, quarant: true},
		{name: "http 200 stream truncated", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, StatusCode: status(http.StatusOK), Message: ErrOpenAIUpstreamStreamTruncated.Error()}, quarant: true},
		{name: "http2 transport", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, Message: "http2: stream closed"}, quarant: true},
		{name: "http 200 tool context error", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, StatusCode: status(http.StatusOK), Message: "No tool output found for custom tool call"}, quarant: false},
		{name: "success", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventSuccess, StatusCode: status(http.StatusOK)}, quarant: false},
		{name: "client canceled", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, Message: "context canceled"}, quarant: false},
		{name: "canceled after headers", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, StatusCode: status(http.StatusOK), Message: "stream usage incomplete: context canceled"}, quarant: false},
		{name: "upstream timeout", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, StatusCode: status(http.StatusGatewayTimeout), Message: "context deadline exceeded"}, quarant: true},
		{name: "request error", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventRequestError, StatusCode: status(http.StatusBadRequest)}, quarant: false},
		{name: "upstream forbidden", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, StatusCode: status(http.StatusForbidden)}, quarant: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.quarant, shouldQuarantineOpenAIAutoCheapestAccount(tt.input))
		})
	}
}

func TestRecordOpenAIAutoSchedulerOutcome_StreamFailureAfterHTTP200(t *testing.T) {
	circuit := &autoCheapestCircuitStub{}
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true, circuit)
	account := &Account{ID: 9, Platform: PlatformOpenAI}
	status := http.StatusOK
	err := newOpenAIUpstreamStreamReadError(ErrOpenAIUpstreamStreamTruncated)
	(&OpenAIGatewayService{}).recordOpenAIAutoSchedulerOutcome(ctx, account, int64PtrForTest(42), "gpt-5.4",
		openAIAutoSchedulerErrorOutcome(time.Now(), &status, err))
	require.Len(t, circuit.keys, 1)
	require.Equal(t, int64(9), circuit.keys[0].AccountID)
	require.Empty(t, circuit.successKeys)
}

func TestRecordOpenAIAutoSchedulerOutcome_UsesStableAccountCircuitKey(t *testing.T) {
	circuit := &autoCheapestCircuitStub{}
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true, circuit)
	setOpenAIAutoCheapestGroupHealthContext(ctx, "gpt-5.4", "responses", "http_sse")
	gatewayService := &OpenAIGatewayService{}
	account := &Account{ID: 9, Platform: PlatformOpenAI}
	status := http.StatusBadGateway

	gatewayService.recordOpenAIAutoSchedulerOutcome(ctx, account, int64PtrForTest(42), "gpt-5.4", OpenAIAutoSchedulerRecordInput{
		EventType:  OpenAIAutoSchedulerEventError,
		StatusCode: &status,
		Message:    "upstream unavailable",
	}, openAIAutoSchedulerAttemptMetadata{Endpoint: "responses", Transport: OpenAIUpstreamTransportHTTPSSE})

	require.Len(t, circuit.keys, 1)
	require.Equal(t, int64(9), circuit.keys[0].AccountID)
	require.Empty(t, circuit.keys[0].Endpoint)
	require.Empty(t, circuit.keys[0].Transport)
}

func TestAutoCheapestSourceFailures_AreRequestLocalAndUsePhysicalPool(t *testing.T) {
	poolID := int64(99)
	apiKey := &APIKey{UserID: 7, GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest}
	keys := []*APIKey{
		CloneAPIKeyForEffectiveGroup(apiKey, &Group{ID: 10, Platform: PlatformOpenAI}),
		CloneAPIKeyForEffectiveGroup(apiKey, &Group{ID: 20, Platform: PlatformOpenAI, SelfHostedPoolGroupID: &poolID, SelfHostedPoolStatus: StatusActive}),
		CloneAPIKeyForEffectiveGroup(apiKey, &Group{ID: 30, Platform: PlatformOpenAI, SelfHostedPoolGroupID: &poolID, SelfHostedPoolStatus: StatusActive}),
	}
	svc := &OpenAIGatewayService{}
	circuit := &autoCheapestCircuitStub{}
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true, circuit)
	noteOpenAIAutoCheapestSelectedSource(ctx, 901, poolID)
	noteOpenAIAutoCheapestSelectedSource(ctx, 101, 10)
	sources := func(ctx context.Context, excluded map[int64]struct{}) []int64 {
		var ids []int64
		for _, stage := range svc.openAIAutoCheapestAccountSourceStages(ctx, keys, excluded) {
			ids = append(ids, *stage.sourceGroupID)
		}
		return ids
	}
	require.Equal(t, []int64{99, 10, 20, 30}, sources(ctx, nil))
	require.Equal(t, []int64{10, 20, 30, 99}, sources(ctx, map[int64]struct{}{901: {}}))
	require.Equal(t, []int64{20, 30, 99, 10}, sources(ctx, map[int64]struct{}{901: {}, 101: {}}))
	// External exclusions do not implicate a source we never selected.
	require.Equal(t, []int64{99, 10, 20, 30}, sources(ctx, map[int64]struct{}{999: {}}))
	fresh := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true, circuit)
	require.Equal(t, []int64{99, 10, 20, 30}, sources(fresh, map[int64]struct{}{901: {}}))
	require.Zero(t, circuit.calls, "one user's failover must not open a global group circuit")
}
