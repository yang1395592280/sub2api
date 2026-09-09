package service

import (
	"context"
	"net/http"
	"testing"

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
		{name: "client canceled", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, Message: "context canceled"}, quarant: false},
		{name: "request error", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventRequestError, StatusCode: status(http.StatusBadRequest)}, quarant: false},
		{name: "upstream forbidden", input: OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError, StatusCode: status(http.StatusForbidden)}, quarant: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.quarant, shouldQuarantineOpenAIAutoCheapestAccount(tt.input))
		})
	}
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
