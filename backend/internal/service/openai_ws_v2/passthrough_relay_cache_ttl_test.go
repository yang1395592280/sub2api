package openai_ws_v2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRelayCacheTTLProgressiveUsageAndTurnTotals(t *testing.T) {
	state := &relayState{}
	parseUsageAndAccumulate(state, []byte(`{"response":{"usage":{"input_tokens_details":{"cache_write_tokens":10,"cache_write_tokens_details":{"ttl_30m_tokens":2,"ttl_12h_tokens":8}}}}}`), "response.in_progress", nil)
	parseUsageAndAccumulate(state, []byte(`{"response":{"usage":{"input_tokens":0,"output_tokens":0}}}`), "response.completed", nil)
	first := finalizeRelayTurnUsage(state)
	require.Equal(t, 2, first.CacheCreation30mInputTokens)
	require.Equal(t, 8, first.CacheCreation12hInputTokens)

	parseUsageAndAccumulate(state, []byte(`{"response":{"usage":{"input_tokens":20,"output_tokens":1,"prompt_tokens_details":{"cache_write_tokens":10,"cache_write_tokens_details":{"ttl_30m_tokens":10,"ttl_12h_tokens":0}}}}}`), "response.completed", nil)
	second := finalizeRelayTurnUsage(state)
	require.Equal(t, 10, second.CacheCreation30mInputTokens)
	require.Zero(t, second.CacheCreation12hInputTokens)

	// A later observation with unknown TTL replaces both buckets with zero.
	parseUsageAndAccumulate(state, []byte(`{"response":{"usage":{"input_tokens_details":{"cache_write_tokens":10,"cache_write_tokens_details":{"ttl_30m_tokens":0,"ttl_12h_tokens":10}}}}}`), "response.in_progress", nil)
	parseUsageAndAccumulate(state, []byte(`{"response":{"usage":{"input_tokens_details":{"cache_write_tokens":5}}}}`), "response.in_progress", nil)
	parseUsageAndAccumulate(state, []byte(`{"response":{"usage":{"input_tokens":0,"output_tokens":0}}}`), "response.completed", nil)
	third := finalizeRelayTurnUsage(state)
	require.Equal(t, 5, third.CacheCreationInputTokens)
	require.Zero(t, third.CacheCreation30mInputTokens)
	require.Zero(t, third.CacheCreation12hInputTokens)

	result := &RelayResult{}
	enrichResult(result, state, time.Second)
	require.Equal(t, 25, result.Usage.CacheCreationInputTokens)
	require.Equal(t, 12, result.Usage.CacheCreation30mInputTokens)
	require.Equal(t, 8, result.Usage.CacheCreation12hInputTokens)
	require.Equal(t, Usage{}, state.turnUsage)
}
