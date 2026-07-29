package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type autoCheapestCircuitStub struct {
	calls int
	keys  []OpenAIAutoCheapestGroupHealthKey
}

func (s *autoCheapestCircuitStub) Allow(context.Context, OpenAIAutoCheapestGroupHealthKey) (bool, error) {
	return true, nil
}
func (s *autoCheapestCircuitStub) RecordFailure(_ context.Context, key OpenAIAutoCheapestGroupHealthKey, _ string) error {
	s.calls++
	s.keys = append(s.keys, key)
	return nil
}
func (s *autoCheapestCircuitStub) RecordSuccess(context.Context, OpenAIAutoCheapestGroupHealthKey) error {
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
