package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type autoCheapestCircuitStub struct{ calls int }

func (s *autoCheapestCircuitStub) Allow(context.Context, OpenAIAutoCheapestGroupHealthKey) (bool, error) { return true, nil }
func (s *autoCheapestCircuitStub) RecordFailure(context.Context, OpenAIAutoCheapestGroupHealthKey, string) error {
	s.calls++
	return nil
}
func (s *autoCheapestCircuitStub) RecordSuccess(context.Context, OpenAIAutoCheapestGroupHealthKey) error { return nil }

func TestMarkOpenAIAutoCheapestGroupFailed_RecordsCircuitOncePerRequest(t *testing.T) {
	circuit := &autoCheapestCircuitStub{}
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true, circuit)

	MarkOpenAIAutoCheapestGroupFailed(ctx, 42, "upstream_502")
	MarkOpenAIAutoCheapestGroupFailed(ctx, 42, "upstream_503")

	require.Equal(t, 1, circuit.calls)
	reason, ok := openAIAutoCheapestGroupFailureReason(ctx, 42)
	require.True(t, ok)
	require.Equal(t, "upstream_502", reason)
}
