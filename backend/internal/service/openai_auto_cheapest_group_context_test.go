package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoCheapestRequestContextSkipsFailedGroup(t *testing.T) {
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true)

	MarkOpenAIAutoCheapestGroupFailed(ctx, 10, "upstream_503")

	reason, failed := openAIAutoCheapestGroupFailureReason(ctx, 10)
	require.True(t, failed)
	require.Equal(t, "upstream_503", reason)
	_, failed = openAIAutoCheapestGroupFailureReason(ctx, 20)
	require.False(t, failed)
}

func TestOpenAIAutoCheapestRequestContextDisabledIsNoop(t *testing.T) {
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), false)

	MarkOpenAIAutoCheapestGroupFailed(ctx, 10, "upstream_503")

	_, failed := openAIAutoCheapestGroupFailureReason(ctx, 10)
	require.False(t, failed)
}
