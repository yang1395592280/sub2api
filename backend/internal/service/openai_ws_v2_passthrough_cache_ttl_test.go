package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIWSPassthroughTurnResultCarriesCacheTTLUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	upstream := newStagedPassthroughConn()
	results := make(chan *OpenAIForwardResult, 4)
	server, _ := startPassthroughHookRecordingServer(t, ctx,
		newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream),
		passthroughLifecycleAccount(), &OpenAIWSIngressHooks{
			AfterTurn: func(_ int, result *OpenAIForwardResult, err error) {
				if result != nil && err == nil {
					results <- result
				}
			},
		})
	defer server.Close()
	client := dialPassthroughLifecycleClient(t, server)
	defer func() { _ = client.CloseNow() }()
	billing := NewBillingService(nil, nil)
	pricing := &ModelPricing{CacheCreationPricePerToken: 1e-6, CacheCreation12hPricePerToken: float64Ptr(2e-6)}

	for i, tc := range []struct {
		name, details string
		short, long   int
		wantCost      float64
	}{
		{"mixed", `,"cache_write_tokens_details":{"ttl_30m_tokens":2,"ttl_12h_tokens":8}`, 2, 8, 18e-6},
		{"short only", `,"cache_write_tokens_details":{"ttl_30m_tokens":10,"ttl_12h_tokens":0}`, 10, 0, 10e-6},
		{"unknown", ``, 0, 0, 10e-6},
		{"invalid", `,"cache_write_tokens_details":{"ttl_30m_tokens":5,"ttl_12h_tokens":8}`, 0, 0, 10e-6},
	} {
		if i > 0 {
			writeCtx, stop := context.WithTimeout(ctx, time.Second)
			err := client.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":[]}`))
			stop()
			require.NoError(t, err)
		}
		requirePassthroughUpstreamWrite(t, upstream, time.Second)
		upstream.Send(fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_ttl_%d","usage":{"input_tokens":20,"output_tokens":1,"input_tokens_details":{"cache_write_tokens":10%s}}}}`, i, tc.details))
		_, err := readPassthroughLifecycleFrame(t, client, time.Second)
		require.NoError(t, err)
		select {
		case result := <-results:
			require.Equal(t, 10, result.Usage.CacheCreationInputTokens, tc.name)
			require.Equal(t, tc.short, result.Usage.CacheCreation30mInputTokens, tc.name)
			require.Equal(t, tc.long, result.Usage.CacheCreation12hInputTokens, tc.name)
			cost := billing.computeTokenBreakdown(pricing, UsageTokens{
				CacheCreationTokens:    result.Usage.CacheCreationInputTokens,
				CacheCreation30mTokens: result.Usage.CacheCreation30mInputTokens,
				CacheCreation12hTokens: result.Usage.CacheCreation12hInputTokens,
			}, 1, "", false)
			require.InDelta(t, tc.wantCost, cost.CacheCreationCost, 1e-12, tc.name)
		case <-time.After(time.Second):
			t.Fatal("missing turn result: " + tc.name)
		}
	}
}
