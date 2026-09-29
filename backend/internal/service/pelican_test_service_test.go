package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPelicanHTMLExtractionRequiresCompleteDocument(t *testing.T) {
	valid := "<!doctype html><html><body><canvas></canvas></body></html>"
	require.Equal(t, valid, extractPelicanHTML(valid))
	require.Equal(t, valid, extractPelicanHTML("```html\n"+valid+"\n```"))
	require.Empty(t, extractPelicanHTML("Here is your page:\n"+valid))
	require.Empty(t, extractPelicanHTML("<html><body>unfinished"))
	require.Empty(t, extractPelicanHTML("```javascript\n"+valid+"\n```"))
}

func TestOpenAIAccountTestPayloadUsesCustomPrompt(t *testing.T) {
	const prompt = "绘制鹈鹕骑自行车的完整 HTML"
	payload := createOpenAITestPayloadWithPrompt("gpt-6-astra", false, prompt)
	input := payload["input"].([]map[string]any)
	content := input[0]["content"].([]map[string]any)
	require.Equal(t, prompt, content[0]["text"])
	require.Equal(t, "hi", createOpenAITestPayload("gpt-6-astra", false)["input"].([]map[string]any)[0]["content"].([]map[string]any)[0]["text"])
}

func TestAnthropicAndGrokAccountTestPayloadsUseCustomPrompt(t *testing.T) {
	const prompt = "绘制鹈鹕骑自行车的完整 HTML"
	anthropic, err := createTestPayloadWithPrompt("claude-sonnet-4", prompt)
	require.NoError(t, err)
	messages := anthropic["messages"].([]map[string]any)
	content := messages[0]["content"].([]map[string]any)
	require.Equal(t, prompt, content[0]["text"])
	require.Equal(t, 8192, anthropic["max_tokens"])

	grokBytes, err := createGrokAccountTestPayload("grok-4.5", prompt)
	require.NoError(t, err)
	var grok map[string]any
	require.NoError(t, json.Unmarshal(grokBytes, &grok))
	require.Equal(t, prompt, grok["input"])
}

func TestPelicanErrorRedactsAccountCredentials(t *testing.T) {
	account := &Account{Credentials: map[string]any{"api_key": "private-api-key", "refresh_token": "private-refresh-token"}}
	redacted := redactPelicanAccountSecrets(account, "upstream rejected private-api-key and private-refresh-token")
	require.NotContains(t, redacted, "private-api-key")
	require.NotContains(t, redacted, "private-refresh-token")
}
