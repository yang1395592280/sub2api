package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoCheapestRequestContextSkipsExhaustedGroup(t *testing.T) {
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true)

	markOpenAIAutoCheapestGroupExhausted(ctx, 10, "no_available_accounts")

	reason, exhausted := openAIAutoCheapestGroupExhaustionReason(ctx, 10)
	require.True(t, exhausted)
	require.Equal(t, "no_available_accounts", reason)
	_, exhausted = openAIAutoCheapestGroupExhaustionReason(ctx, 20)
	require.False(t, exhausted)
}

func TestOpenAIAutoCheapestRequestContextDisabledIsNoop(t *testing.T) {
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), false)

	markOpenAIAutoCheapestGroupExhausted(ctx, 10, "no_available_accounts")

	_, exhausted := openAIAutoCheapestGroupExhaustionReason(ctx, 10)
	require.False(t, exhausted)
}

func TestMarkOpenAIAutoCheapestGroupExhaustedIfNeededOnlyMarksEmptySelection(t *testing.T) {
	circuit := &autoCheapestCircuitStub{}
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true, circuit)
	account := &Account{ID: 99}

	markOpenAIAutoCheapestGroupExhaustedIfNeeded(ctx, 10, &AccountSelectionResult{Account: account, Acquired: true}, nil)
	require.Equal(t, 0, circuit.calls)

	markOpenAIAutoCheapestGroupExhaustedIfNeeded(ctx, 10, nil, ErrNoAvailableAccounts)
	require.Equal(t, 1, circuit.calls)
	reason, exhausted := openAIAutoCheapestGroupExhaustionReason(ctx, 10)
	require.True(t, exhausted)
	require.Equal(t, "no_available_accounts", reason)
}
