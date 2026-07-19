package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type autoCheapestCircuitStub struct{ calls int }

func (s *autoCheapestCircuitStub) Allow(context.Context, OpenAIAutoCheapestGroupHealthKey) (bool, error) {
	return true, nil
}
func (s *autoCheapestCircuitStub) RecordFailure(context.Context, OpenAIAutoCheapestGroupHealthKey, string) error {
	s.calls++
	return nil
}
func (s *autoCheapestCircuitStub) RecordSuccess(context.Context, OpenAIAutoCheapestGroupHealthKey) error {
	return nil
}

func TestMarkOpenAIAutoCheapestGroupExhausted_RecordsCircuitOncePerRequest(t *testing.T) {
	circuit := &autoCheapestCircuitStub{}
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true, circuit)

	markOpenAIAutoCheapestGroupExhausted(ctx, 42, "no_available_accounts")
	markOpenAIAutoCheapestGroupExhausted(ctx, 42, "no_available_accounts")

	require.Equal(t, 1, circuit.calls)
	reason, ok := openAIAutoCheapestGroupExhaustionReason(ctx, 42)
	require.True(t, ok)
	require.Equal(t, "no_available_accounts", reason)
}
