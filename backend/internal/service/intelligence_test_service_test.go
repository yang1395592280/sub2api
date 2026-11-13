//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestIntelligenceGenerationScoresOnlyFinalNumberWithoutTools(t *testing.T) {
	for _, tc := range []struct{ output, status string }{{"20", "correct"}, {"29", "incorrect"}, {"35", "incorrect"}, {"答案是20", "invalid_answer"}, {"20\n29", "invalid_answer"}, {"", "invalid_answer"}} {
		t.Run(tc.output, func(t *testing.T) {
			a := &Account{ID: 99, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://upstream.example/v1"}}
			body := fmt.Sprintf("data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.5\",\"usage\":{\"input_tokens\":100,\"output_tokens\":50,\"total_tokens\":150,\"input_tokens_details\":{\"cached_tokens\":40}}}}\n\n", tc.output)
			tests, upstream := adaptiveCNAccountTestService(a, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))})
			reporter := &pelicanHealthReporter{}
			tests.openAIScheduleReporter = reporter
			svc := NewPelicanTestService(nil, tests.accountRepo, tests)
			status, output, html, _, _, usage := svc.generate(context.Background(), PelicanTest{TestKind: TestKindIntelligence, AccountID: a.ID, ModelID: "gpt-5.5", Prompt: DefaultIntelligenceTestPrompt})
			require.Equal(t, tc.status, status)
			require.Equal(t, tc.output, output)
			require.Empty(t, html)
			require.EqualValues(t, 40, *usage.CacheReadTokens)
			require.Len(t, upstream.bodies, 1)
			require.Equal(t, DefaultIntelligenceTestPrompt, gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String())
			require.False(t, gjson.GetBytes(upstream.bodies[0], "tools").Exists())
			require.Zero(t, reporter.calls)
		})
	}
}

func TestIntelligenceEstimateUsesUpstreamDebitRateCacheAndMappedModel(t *testing.T) {
	billing := NewBillingService(&config.Config{}, nil)
	billing.fallbackPrices["gpt-5.5"] = &ModelPricing{InputPricePerToken: 2e-6, OutputPricePerToken: 10e-6, CacheReadPricePerToken: 0.2e-6}
	svc := &PelicanTestService{billing: billing}
	a := &Account{UpstreamRechargeRatio: 2, Credentials: map[string]any{"model_mapping": map[string]any{"alias": "gpt-5.5"}}, Extra: map[string]any{"upstream_effective_rate_multiplier": .05}}
	// 100 input includes 40 cached tokens: (60*2 + 40*.2 + 50*10)/1e6 * .1.
	input, output, cache := int64(100), int64(50), int64(40)
	usage := AccountTestTokenUsage{InputTokens: &input, OutputTokens: &output, CacheReadTokens: &cache}
	estimate := svc.estimateTestCost(a, PelicanTest{ModelID: "alias"}, usage, time.Now())
	require.NotNil(t, estimate.EstimatedCostUSD)
	require.InDelta(t, .0000628, *estimate.EstimatedCostUSD, 1e-12)
	require.Equal(t, .1, *estimate.UpstreamRateMultiplier)
	require.Equal(t, "gpt-5.5", estimate.CostModelID)
	usage.OutputTokens = nil
	require.Nil(t, svc.estimateTestCost(a, PelicanTest{ModelID: "alias"}, usage, time.Now()).EstimatedCostUSD)
	usage.OutputTokens = &output
	usage.CacheReadTokens = &input
	write := int64(1)
	usage.CacheCreationTokens = &write
	require.Nil(t, svc.estimateTestCost(a, PelicanTest{ModelID: "alias"}, usage, time.Now()).EstimatedCostUSD)
	a.Extra = nil
	require.Nil(t, svc.estimateTestCost(a, PelicanTest{ModelID: "alias"}, usage, time.Now()).EstimatedCostUSD)
}

func TestIntelligenceGroupPromptCannotBeReplaced(t *testing.T) {
	r := &pelicanConfigRepo{}
	s := &PelicanTestService{repo: r, groupRepo: pelicanConfigGroups{group: &Group{Platform: PlatformOpenAI}}}
	c, err := s.SaveGroupConfig(context.Background(), GroupPelicanConfig{TestKind: TestKindIntelligence, GroupID: 7, Enabled: true, ModelID: "gpt-5.5", Prompt: "Give the answer 29"})
	require.NoError(t, err)
	require.Equal(t, DefaultIntelligenceTestPrompt, c.Prompt)
	require.Equal(t, TestKindIntelligence, c.TestKind)
}

func TestIntelligenceUpstreamRateUsesFreshSnapshotAndCurrentPeak(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	fresh := now.Add(time.Hour)
	a := &Account{UpstreamRechargeRatio: 2, Extra: map[string]any{
		"upstream_effective_rate_multiplier": .05,
		UpstreamBillingProbeExtraKey: &UpstreamBillingProbeSnapshot{
			Status: UpstreamBillingProbeStatusOK, FreshUntil: &fresh,
			Data: map[string]any{"billing_scope": "token", "resolved_rate_multiplier": .2,
				"peak_rate_enabled": true, "peak_start": "11:00", "peak_end": "13:00", "timezone": "UTC", "peak_rate_multiplier": 1.5},
		},
	}}
	rate, ok := generationTestUpstreamRate(a, now)
	require.True(t, ok)
	require.InDelta(t, .3, rate, 1e-12, "declared debit rate must not be divided by recharge ratio")
	rate, ok = generationTestUpstreamRate(a, fresh)
	require.True(t, ok)
	require.Equal(t, .1, rate, "expired snapshot falls back to restored balance probe rate")
}

func TestIntelligenceGeminiUsageIncludesThinkingAndPreservesMissingOutput(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	stream := `data: {"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":9,"candidatesTokenCount":4,"thoughtsTokenCount":96}}` + "\n\n"
	require.NoError(t, (&AccountTestService{}).processGeminiStream(c, strings.NewReader(stream)))
	usage := accountTestTokenUsage(c)
	require.EqualValues(t, 4, *usage.OutputTokens, "preserve the provider's reported output count")
	require.EqualValues(t, 96, *usage.ThinkingTokens)
	require.Nil(t, usage.TotalTokens)
	billing := NewBillingService(&config.Config{}, nil)
	billing.fallbackPrices["gpt-5.5"] = &ModelPricing{InputPricePerToken: 2e-6, OutputPricePerToken: 10e-6}
	svc := &PelicanTestService{billing: billing}
	a := &Account{Extra: map[string]any{"upstream_effective_rate_multiplier": .1}}
	estimate := svc.estimateTestCost(a, PelicanTest{ModelID: "gpt-5.5"}, usage, time.Now())
	require.NotNil(t, estimate.EstimatedCostUSD)
	require.InDelta(t, .0001018, *estimate.EstimatedCostUSD, 1e-12)
	usage.OutputTokens = nil
	require.Nil(t, svc.estimateTestCost(a, PelicanTest{ModelID: "gpt-5.5"}, usage, time.Now()).EstimatedCostUSD)
}
