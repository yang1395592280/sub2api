package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesUsageCacheTTLMetadata(t *testing.T) {
	var usage ResponsesUsage
	require.NoError(t, json.Unmarshal([]byte(`{"input_tokens":20,"output_tokens":1,"input_tokens_details":{"cache_write_tokens":10,"cache_write_tokens_details":{"ttl_30m_tokens":2,"ttl_12h_tokens":8}}}`), &usage))
	require.Equal(t, 2, usage.CacheCreation30mInputTokens)
	require.Equal(t, 8, usage.CacheCreation12hInputTokens)
	encoded, err := json.Marshal(usage)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "cache_creation_30m_input_tokens")
	require.NotContains(t, string(encoded), "cache_creation_12h_input_tokens")

	for _, details := range []string{
		``,
		`,"cache_write_tokens_details":{"ttl_12h_tokens":10}`,
		`,"cache_write_tokens_details":{"ttl_30m_tokens":3,"ttl_12h_tokens":8}`,
		`,"cache_write_tokens_details":{"ttl_30m_tokens":0.5,"ttl_12h_tokens":9.5}`,
	} {
		require.NoError(t, json.Unmarshal([]byte(`{"input_tokens":20,"output_tokens":1,"input_tokens_details":{"cache_write_tokens":10`+details+`}}`), &usage))
		require.Equal(t, 10, usage.CacheCreationInputTokens)
		require.Zero(t, usage.CacheCreation30mInputTokens)
		require.Zero(t, usage.CacheCreation12hInputTokens)
	}
}
