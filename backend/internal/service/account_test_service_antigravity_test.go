//go:build unit

package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestService_AntigravityPromptImagesAndTiming(t *testing.T) {
	upstream := &queuedHTTPUpstreamStub{
		responses: []*http.Response{{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Here is your image\"},{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"iVBORw0KGgo=\"}}]}}]}}\n\n",
			)),
		}},
	}
	svc := &AccountTestService{antigravityGatewayService: &AntigravityGatewayService{
		tokenProvider: &AntigravityTokenProvider{},
		httpUpstream:  upstream,
	}}
	account := &Account{
		ID: 1, Platform: PlatformAntigravity, Type: AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "test-token", "project_id": "test-project",
			"model_mapping": map[string]any{"gemini-3.1-flash-image": "gemini-3.1-flash-image"},
		},
	}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)

	require.NoError(t, svc.routeAntigravityTest(c, account, "gemini-3.1-flash-image", "draw a blue bird"))
	require.Len(t, upstream.requestBodies, 1)
	var request struct {
		Request struct {
			Contents []struct {
				Parts []struct{ Text string }
			}
		}
	}
	require.NoError(t, json.Unmarshal(upstream.requestBodies[0], &request))
	require.Len(t, request.Request.Contents, 1)
	require.Len(t, request.Request.Contents[0].Parts, 1)
	require.Equal(t, "draw a blue bird", request.Request.Contents[0].Parts[0].Text)

	var events []TestEvent
	for _, line := range strings.Split(writer.Body.String(), "\n") {
		if payload, ok := strings.CutPrefix(line, "data: "); ok {
			var event TestEvent
			require.NoError(t, json.Unmarshal([]byte(payload), &event))
			events = append(events, event)
		}
	}
	require.Len(t, events, 5)
	require.Equal(t, "test_start", events[0].Type)
	require.Equal(t, "connect_timing", events[1].Type)
	require.Positive(t, events[1].ConnectDurationMS)
	require.Equal(t, "content", events[2].Type)
	require.Equal(t, "Here is your image", events[2].Text)
	require.Equal(t, "image", events[3].Type)
	require.Equal(t, "data:image/png;base64,iVBORw0KGgo=", events[3].ImageURL)
	require.Equal(t, "image/png", events[3].MimeType)
	require.Equal(t, "test_complete", events[4].Type)
	require.True(t, events[4].Success)
}

func TestAntigravityConnectionTestModel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "claude-sonnet-4-6", antigravityConnectionTestModel(""))
	require.Equal(t, "gemini-3.1-pro-preview", antigravityConnectionTestModel("gemini-3.1-pro-preview"))
}

func TestAntigravityBuildGeminiTestRequest_IncludesValidatedToolConfig(t *testing.T) {
	t.Parallel()

	svc := &AntigravityGatewayService{}
	raw, err := svc.buildGeminiTestRequestWithPrompt("proj-123", "gemini-3.8-flash-high", "")
	require.NoError(t, err)

	var v1Req map[string]any
	require.NoError(t, json.Unmarshal(raw, &v1Req))
	require.Equal(t, "proj-123", v1Req["project"])
	require.Equal(t, "gemini-3.8-flash-high", v1Req["model"])

	inner, ok := v1Req["request"].(map[string]any)
	require.True(t, ok)
	toolCfg, ok := inner["toolConfig"].(map[string]any)
	require.True(t, ok, "expected toolConfig to be present in Gemini test request")
	fcCfg, ok := toolCfg["functionCallingConfig"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "VALIDATED", fcCfg["mode"])
}

func TestAntigravityBuildGeminiTestRequestWithPrompt_ImageModel(t *testing.T) {
	t.Parallel()

	svc := &AntigravityGatewayService{}
	raw, err := svc.buildGeminiTestRequestWithPrompt("proj-123", "gemini-3.1-flash-image", "a cute cat")
	require.NoError(t, err)

	var v1Req map[string]any
	require.NoError(t, json.Unmarshal(raw, &v1Req))
	inner, ok := v1Req["request"].(map[string]any)
	require.True(t, ok)

	genCfg, ok := inner["generationConfig"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{"TEXT", "IMAGE"}, genCfg["responseModalities"])

	contents, ok := inner["contents"].([]any)
	require.True(t, ok)
	require.Len(t, contents, 1)
	firstContent := contents[0].(map[string]any)
	parts := firstContent["parts"].([]any)
	require.Equal(t, "a cute cat", parts[0].(map[string]any)["text"])
}

func TestExtractContentFromSSEResponse_ExtractsTextAndInlineImages(t *testing.T) {
	t.Parallel()

	sse := "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Here is your image\"},{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"iVBORw0KGgo=\"}}]}}]}}\n\n"
	text, images := extractContentFromSSEResponse([]byte(sse))
	require.Equal(t, "Here is your image", text)
	require.Len(t, images, 1)
	require.Equal(t, "image/png", images[0].MimeType)
	require.Equal(t, "data:image/png;base64,iVBORw0KGgo=", images[0].ImageURL)
}

func TestAntigravityShouldUseGeminiTestBuilder_UsesMappedModel(t *testing.T) {
	t.Parallel()

	require.True(t, shouldUseAntigravityGeminiTestRequest("custom-alias", "gemini-2.5-flash"))
	require.True(t, shouldUseAntigravityGeminiTestRequest("gemini-3.1-pro", "gemini-pro-agent"))
	require.False(t, shouldUseAntigravityGeminiTestRequest("gemini-alias", "claude-sonnet-4-6"))
	require.False(t, shouldUseAntigravityGeminiTestRequest("claude-sonnet-4-6", "claude-sonnet-4-6"))
}

// Every other platform's connection test sends the admin's prompt (default "hi") and lets the
// model answer. The Antigravity text test sent "." with maxOutputTokens 1, ignoring the prompt,
// so thinking models (gemini-4-argon, gemini-3.8-flash) spent that token thinking and the test
// showed "sent: hi" with an empty response.
func TestAntigravityBuildGeminiTestRequest_UsesPromptAndRoomToAnswer(t *testing.T) {
	t.Parallel()

	svc := &AntigravityGatewayService{}
	for _, tc := range []struct{ prompt, want string }{{"hello there", "hello there"}, {"", "hi"}, {"   ", "hi"}} {
		raw, err := svc.buildGeminiTestRequestWithPrompt("proj-123", "gemini-4-argon-high", tc.prompt)
		require.NoError(t, err)
		var v1Req map[string]any
		require.NoError(t, json.Unmarshal(raw, &v1Req))
		inner := v1Req["request"].(map[string]any)
		parts := inner["contents"].([]any)[0].(map[string]any)["parts"].([]any)
		require.Equal(t, tc.want, parts[0].(map[string]any)["text"], "prompt %q", tc.prompt)
		genCfg := inner["generationConfig"].(map[string]any)
		require.EqualValues(t, antigravityTestMaxOutputTokens, genCfg["maxOutputTokens"])
		require.GreaterOrEqual(t, antigravityTestMaxOutputTokens, 128, "a thinking model needs room to answer")
	}
}

func TestAntigravityBuildClaudeTestRequest_UsesPromptAndRoomToAnswer(t *testing.T) {
	t.Parallel()

	svc := &AntigravityGatewayService{}
	raw, err := svc.buildClaudeTestRequest("proj-123", "claude-sonnet-4-6", "hello there")
	require.NoError(t, err)
	require.Contains(t, string(raw), "hello there")
	require.NotContains(t, string(raw), `"text":"."`)

	raw, err = svc.buildClaudeTestRequest("proj-123", "claude-sonnet-4-6", "")
	require.NoError(t, err)
	require.Contains(t, string(raw), `"hi"`)
}
