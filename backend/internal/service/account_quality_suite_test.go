package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	quality "github.com/Wei-Shaw/sub2api/internal/accountquality"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type suiteAccounts struct{ qualityScopeAccounts }

func (r suiteAccounts) GetByID(_ context.Context, id int64) (*Account, error) {
	for _, a := range r.items {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, fmt.Errorf("missing test account")
}

type suiteUpstream struct {
	HTTPUpstream
	requests []map[string]any
	review   string
}

func (u *suiteUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	var payload map[string]any
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		return nil, err
	}
	u.requests = append(u.requests, payload)
	encoded, _ := json.Marshal(payload)
	text := `{"answer":21}`
	if len(u.requests) == 2 {
		nonce := regexp.MustCompile(`校验码：([a-f0-9-]+)`).FindStringSubmatch(string(encoded))
		if len(nonce) != 2 {
			return nil, fmt.Errorf("nonce missing")
		}
		text = `<svg xmlns="http://www.w3.org/2000/svg"><text>` + nonce[1] + `</text><circle r="10"><animate attributeName="cx" values="0;10;0" dur="3s" repeatCount="indefinite"/></circle></svg>`
	}
	if len(u.requests) == 3 {
		text = u.review
	}
	usage := map[string]any{"input_tokens": 200, "output_tokens": 90, "total_tokens": 290, "output_tokens_details": map[string]any{"reasoning_tokens": 32}}
	response := map[string]any{"object": "response", "status": "completed", "usage": usage, "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text}}}}}
	if _, chat := payload["messages"]; chat {
		response = map[string]any{"usage": map[string]any{"prompt_tokens": 200, "completion_tokens": 90, "total_tokens": 290, "completion_tokens_details": map[string]any{"reasoning_tokens": 32}}, "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": text}}}}
	}
	raw, _ := json.Marshal(response)
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(raw))}, nil
}

func TestQualityDualSuiteUsesIndependentPromptsAndReviewerImages(t *testing.T) {
	for _, protocol := range []string{"responses", "chat_completions"} {
		t.Run(protocol, func(t *testing.T) {
			var pngData bytes.Buffer
			require.NoError(t, png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 960, 640))))
			frames := []quality.Frame{}
			for _, at := range []float64{0, .69, 1.53, 2.37} {
				frames = append(frames, quality.Frame{Time: at, PNG: base64.StdEncoding.EncodeToString(pngData.Bytes())})
			}
			renderer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/render", r.URL.Path)
				require.Equal(t, "Bearer test-renderer", r.Header.Get("Authorization"))
				_ = json.NewEncoder(w).Encode(frames)
			}))
			defer renderer.Close()
			mode := openai_compat.ResponsesSupportModeForceResponses
			if protocol == "chat_completions" {
				mode = openai_compat.ResponsesSupportModeForceChatCompletions
			}
			a := &Account{ID: 42, Name: "tested account", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, GroupIDs: []int64{7}, Credentials: map[string]any{"api_key": "local-fake-key", "base_url": "https://example.test"}, Extra: map[string]any{openai_compat.ExtraKeyResponsesMode: string(mode)}}
			upstream := &suiteUpstream{review: `{"checks":{"pelican":true,"bicycle":true,"riding":false,"motion":true}}`}
			p := &accountQualityProvider{accounts: suiteAccounts{qualityScopeAccounts{items: []*Account{a}}}, groups: qualityScopeGroups{items: []Group{{ID: 7, Name: "0.1", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard}}, members: map[int64][]int64{7: {42}}}, upstream: upstream, concurrency: &ConcurrencyService{}, tls: &TLSFingerprintProfileService{}, renderer: quality.NewFrameRenderer(renderer.URL, "test-renderer")}
			c := quality.Condition{ModelID: "gpt-5.5", Protocol: protocol, Transport: "http", ReasoningEffort: "upstream_default", ServiceTier: "upstream_default", BenchmarkID: "dual", BenchmarkVersion: "1", ScorerVersion: "dual-visual-v1"}
			target, err := p.Resolve(context.Background(), 42, 7, c)
			require.NoError(t, err)
			require.True(t, target.CanProbe)
			policy := quality.Policy{AccountID: 42, SourceGroupID: 7, Rules: quality.DefaultRules(100)}
			policy.SetTarget(target)
			reviewer := c
			reviewer.BenchmarkID = "candy"
			reviewer.ScorerVersion = "strict-json-v1"
			policy.Reviewer = &quality.ReviewerConfig{AccountID: 42, SourceGroupID: 7, Condition: reviewer}
			stages := []string{}
			r := p.Probe(context.Background(), quality.Run{AccountID: 42, Snapshot: policy, ScopeHash: policy.ScopeHash}, func(stage string) error { stages = append(stages, stage); return nil })
			require.Equal(t, []string{"candy", "pelican", "review"}, stages)
			require.Equal(t, "incorrect", r.Verdict)
			require.Equal(t, "passed", r.Candy.Verdict)
			require.Equal(t, "visual_failed", r.Pelican.ReasonCode)
			require.False(t, *r.Review.Checks["riding"])
			require.EqualValues(t, 96, *r.ReasoningTokens)
			require.EqualValues(t, 870, *r.TotalTokens)
			require.NotEmpty(t, r.Evidence.SVG)
			first, _ := json.Marshal(upstream.requests[0])
			second, _ := json.Marshal(upstream.requests[1])
			third, _ := json.Marshal(upstream.requests[2])
			require.Contains(t, string(first), "糖果")
			require.NotContains(t, string(first), "自行车")
			require.Contains(t, string(second), "自行车")
			require.NotContains(t, string(second), "糖果")
			require.Equal(t, 4, strings.Count(string(third), "data:image/png;base64,"))
			require.NotContains(t, string(third), "local-fake-key")
		})
	}
}

func TestQualityCollectorSeparatesReasoningAndFinalAnswer(t *testing.T) {
	c := qualityCollector{protocol: "responses"}
	c.consume([]byte(`{"type":"response.reasoning_text.delta","delta":"answer 21"}`))
	c.consume([]byte(`{"type":"response.output_text.delta","delta":"{\"answer\":20}"}`))
	c.consume([]byte(`{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":10,"output_tokens":50,"output_tokens_details":{"reasoning_tokens":40}}}}`))
	require.Equal(t, `{"answer":20}`, c.text.String())
	require.EqualValues(t, 40, *c.reasoning)
	require.Equal(t, "incorrect", quality.Score(c.text.String(), c.complete).Verdict)
}
