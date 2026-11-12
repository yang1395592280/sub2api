//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func TestConvertedResponsesPreserveCacheTTL(t *testing.T) {
	for _, endpoint := range []string{"chat/completions", "messages"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", endpoint, stream), func(t *testing.T) {
				body := []byte(fmt.Sprintf(`{"model":"gpt-5.4","stream":%t,"max_tokens":32,"messages":[{"role":"user","content":"hello"}]}`, stream))
				response := `data: {"type":"response.completed","response":{"id":"resp_ttl","model":"gpt-5.4","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":20,"output_tokens":1,"input_tokens_details":{"cache_write_tokens":10,"cache_write_tokens_details":{"ttl_30m_tokens":2,"ttl_12h_tokens":8}}}}}` + "\n\n"
				upstream := &httpUpstreamRecorder{resp: &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}},
					Body: io.NopCloser(strings.NewReader(response)),
				}}
				svc := &OpenAIGatewayService{cfg: structuredInputRetryTestConfig(), httpUpstream: upstream}
				c := adaptiveProtocolTestContext("/v1/"+endpoint, body)
				account := structuredInputRetryTestAccount()
				var result *OpenAIForwardResult
				var err error
				if endpoint == "messages" {
					result, err = svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
				} else {
					result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "/v1/responses", upstream.lastReq.URL.Path)
				require.Equal(t, 10, result.Usage.CacheCreationInputTokens)
				require.Equal(t, 2, result.Usage.CacheCreation30mInputTokens)
				require.Equal(t, 8, result.Usage.CacheCreation12hInputTokens)
			})
		}
	}
}

func TestCopyOpenAIUsageFromResponsesUsagePreservesCacheTTL(t *testing.T) {
	for _, path := range []string{"input_tokens_details", "prompt_tokens_details"} {
		t.Run(path, func(t *testing.T) {
			var usage apicompat.ResponsesUsage
			require.NoError(t, json.Unmarshal([]byte(`{"input_tokens":20,"output_tokens":1,"`+path+`":{"cached_tokens":3,"cache_write_tokens":10,"cache_write_tokens_details":{"ttl_30m_tokens":2,"ttl_12h_tokens":8}}}`), &usage))
			got := copyOpenAIUsageFromResponsesUsage(&usage)
			require.Equal(t, 10, got.CacheCreationInputTokens)
			require.Equal(t, 2, got.CacheCreation30mInputTokens)
			require.Equal(t, 8, got.CacheCreation12hInputTokens)
			require.Equal(t, 3, got.CacheReadInputTokens)
		})
	}
}

func TestCopyOpenAIUsageFromResponsesUsageTrustsCanonicalCacheCreationValue(t *testing.T) {
	usage := &apicompat.ResponsesUsage{
		InputTokens:              20,
		OutputTokens:             2,
		CacheCreationInputTokens: 0,
		InputTokensDetails: &apicompat.ResponsesInputTokensDetails{
			CachedTokens:     3,
			CacheWriteTokens: 19,
		},
	}

	got := copyOpenAIUsageFromResponsesUsage(usage)

	require.Equal(t, 20, got.InputTokens)
	require.Equal(t, 3, got.CacheReadInputTokens)
	require.Zero(t, got.CacheCreationInputTokens)
}
