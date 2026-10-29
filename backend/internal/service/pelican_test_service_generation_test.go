//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func pelicanHTMLTestResponse(protocol string) *http.Response {
	var body string
	switch protocol {
	case APIProtocolAnthropic:
		body = "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"<!doctype html><html><body>ok</body></html>\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n"
	case APIProtocolResponses:
		body = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"<!doctype html><html><body>ok</body></html>\"}\n\ndata: {\"type\":\"response.completed\"}\n\n"
	default:
		body = "data: {\"choices\":[{\"delta\":{\"content\":\"<!doctype html><html><body>ok</body></html>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestPelicanGenerationSupportsProfileProviders(t *testing.T) {
	for _, tc := range []struct {
		name, platform, model, protocol, upstreamProtocol string
	}{
		{"opencode default", PlatformOpenCodeGo, DefaultOpenCodeGoTestModel, "", APIProtocolChatCompletions},
		{"command code default", PlatformCommandCode, DefaultCommandCodeTestModel, "", APIProtocolChatCompletions},
		{"command code anthropic", PlatformCommandCode, "claude-sonnet-4-6", "", APIProtocolAnthropic},
		{"command code responses", PlatformCommandCode, "gpt-5.5", "", APIProtocolResponses},
		{"command code pinned chat", PlatformCommandCode, "vendor/custom-model", APIProtocolChatCompletions, APIProtocolChatCompletions},
		{"cline default", PlatformCline, DefaultClineTestModel, "", APIProtocolChatCompletions},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{ID: 8411, Platform: tc.platform, Type: AccountTypeAPIKey,
				Status: StatusActive, Concurrency: 1, Credentials: map[string]any{"api_key": "sk-test"}}
			if tc.protocol != "" {
				account.Credentials["api_protocol"] = tc.protocol
			}
			accountTests, upstream := adaptiveCNAccountTestService(account, pelicanHTMLTestResponse(tc.upstreamProtocol))
			svc := &PelicanTestService{accountRepo: accountTests.accountRepo, accountTest: accountTests}
			const prompt = "Generate a complete HTML document"
			status, output, html, reason, _, _ := svc.generate(context.Background(), PelicanTest{AccountID: account.ID, ModelID: tc.model, Prompt: prompt})
			require.Equal(t, "previewable", status, reason)
			require.Contains(t, output, "</html>")
			require.Equal(t, "<!doctype html><html><body>ok</body></html>", html)
			require.Len(t, upstream.requests, 1, "one generation request per account")
			require.Contains(t, string(upstream.bodies[0]), prompt)
			require.Equal(t, tc.model, gjson.GetBytes(upstream.bodies[0], "model").String())
		})
	}
}

func TestPelicanCustomModelHonorsAccountMapping(t *testing.T) {
	account := &Account{ID: 8412, Platform: PlatformCline, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "model_mapping": map[string]any{"custom-alias": "vendor/upstream-model"}}}
	accountTests, upstream := adaptiveCNAccountTestService(account, pelicanHTMLTestResponse(APIProtocolChatCompletions))
	svc := &PelicanTestService{accountRepo: accountTests.accountRepo, accountTest: accountTests}
	status, _, _, reason, _, _ := svc.generate(context.Background(), PelicanTest{AccountID: account.ID, ModelID: "custom-alias", Prompt: "Generate HTML"})
	require.Equal(t, "previewable", status, reason)
	require.Equal(t, "vendor/upstream-model", gjson.GetBytes(upstream.bodies[0], "model").String())

	status, _, _, reason, _, _ = svc.generate(context.Background(), PelicanTest{AccountID: account.ID, ModelID: "not-configured", Prompt: "Generate HTML"})
	require.Equal(t, "failed", status)
	require.Equal(t, "账号未配置所选模型", reason)
	require.Len(t, upstream.requests, 1, "manual models must still honor the configured whitelist")
}

func TestPelicanGenerationRejectsUnimplementedPromptRoutes(t *testing.T) {
	account := adaptiveCNAccountTestAccount(8413, PlatformDeepseek)
	accountTests, upstream := adaptiveCNAccountTestService(account)
	svc := &PelicanTestService{accountRepo: accountTests.accountRepo, accountTest: accountTests}
	status, _, _, reason, _, _ := svc.generate(context.Background(), PelicanTest{AccountID: account.ID, ModelID: "deepseek-chat", Prompt: "Generate HTML"})
	require.Equal(t, "failed", status)
	require.Equal(t, "该账号协议暂不支持自定义 HTML 生成提示词", reason)
	require.Empty(t, upstream.requests, "adaptive connectivity tests must not silently make several generation requests")
}

type scheduledPelicanAllowed struct {
	PelicanTestRepository
	allowed bool
}

func (r scheduledPelicanAllowed) ScheduledTestEnabled(context.Context, PelicanTest) (bool, error) {
	return r.allowed, nil
}

type pelicanHealthReporter struct{ calls int }

func (r *pelicanHealthReporter) ReportOpenAIAccountScheduleResult(int64, string, bool, *int) {
	r.calls++
}
func TestScheduledPelicanDoesNotWriteAccountHealthOrScoring(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusTooManyRequests} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			a := &Account{ID: 8420, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "sk-test-key", "base_url": "https://upstream.example/v1"}}
			tests, upstream := adaptiveCNAccountTestService(a, &http.Response{StatusCode: code, Header: http.Header{"X-Ratelimit-Reset-Requests": []string{"30s"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"failed"}}`))})
			reporter := &pelicanHealthReporter{}
			tests.openAIScheduleReporter = reporter
			svc := &PelicanTestService{repo: scheduledPelicanAllowed{allowed: true}, accountRepo: tests.accountRepo, accountTest: tests}
			status, _, _, _, _, _ := svc.generate(context.Background(), PelicanTest{AccountID: a.ID, SourceGroupID: 7, ModelID: "gpt-5.5", Prompt: "Generate HTML"})
			require.Equal(t, "failed", status)
			require.Len(t, upstream.requests, 1)
			require.Zero(t, reporter.calls)
			repo := tests.accountRepo.(*openAIAccountTestRepo)
			require.Empty(t, repo.setErrorID)
			require.Empty(t, repo.rateLimitedID)
		})
	}
}
func TestScheduledPelicanRechecksGroupBeforeAnyUpstreamCall(t *testing.T) {
	a := &Account{ID: 8421, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	tests, upstream := adaptiveCNAccountTestService(a)
	svc := &PelicanTestService{repo: scheduledPelicanAllowed{allowed: false}, accountRepo: tests.accountRepo, accountTest: tests}
	status, _, _, reason, _, _ := svc.generate(context.Background(), PelicanTest{AccountID: a.ID, SourceGroupID: 7})
	require.Equal(t, "failed", status)
	require.Contains(t, reason, "已关闭")
	require.Empty(t, upstream.requests)
}
