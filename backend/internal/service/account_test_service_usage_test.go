package service

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestStreamsCaptureReportedTokenUsage(t *testing.T) {
	tests := []struct {
		name   string
		stream string
		read   func(*AccountTestService, *gin.Context, io.Reader) error
		input  int64
		output int64
		total  int64
	}{
		{
			name:   "OpenAI Responses",
			stream: "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":12,\"output_tokens\":8,\"total_tokens\":20}}}\n\n",
			read:   (*AccountTestService).processOpenAIStream,
			input:  12, output: 8, total: 20,
		},
		{
			name: "Anthropic with cached input",
			stream: "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":8,\"cache_read_input_tokens\":2,\"cache_creation_input_tokens\":1,\"output_tokens\":0}}}\n\n" +
				"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":3}}\n\n" +
				"data: {\"type\":\"message_stop\"}\n\n",
			read:  (*AccountTestService).processClaudeStream,
			input: 11, output: 3, total: 14,
		},
		{
			name:   "Gemini",
			stream: "data: {\"candidates\":[{\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":9,\"candidatesTokenCount\":4,\"totalTokenCount\":13}}\n\n",
			read:   (*AccountTestService).processGeminiStream,
			input:  9, output: 4, total: 13,
		},
		{
			name: "Chat Completions",
			stream: "data: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n\n" +
				"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":5,\"total_tokens\":12}}\n\n" +
				"data: [DONE]\n\n",
			read:  (*AccountTestService).processOpenAIChatCompletionsStream,
			input: 7, output: 5, total: 12,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			require.NoError(t, tt.read(&AccountTestService{}, c, strings.NewReader(tt.stream)))
			usage := accountTestTokenUsage(c)
			require.NotNil(t, usage.InputTokens)
			require.NotNil(t, usage.OutputTokens)
			require.NotNil(t, usage.TotalTokens)
			require.Equal(t, tt.input, *usage.InputTokens)
			require.Equal(t, tt.output, *usage.OutputTokens)
			require.Equal(t, tt.total, *usage.TotalTokens)
		})
	}
}

func TestAccountTestStreamWithoutUsageDoesNotEstimateTokens(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	stream := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{}}\n\n"
	require.NoError(t, (&AccountTestService{}).processOpenAIStream(c, strings.NewReader(stream)))
	require.Equal(t, AccountTestTokenUsage{}, accountTestTokenUsage(c))
}

func TestGeminiStreamWithoutTotalDoesNotOmitThoughtTokensFromReportedTotal(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	stream := "data: {\"candidates\":[{\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":9,\"candidatesTokenCount\":4,\"thoughtsTokenCount\":6}}\n\n"
	require.NoError(t, (&AccountTestService{}).processGeminiStream(c, strings.NewReader(stream)))
	usage := accountTestTokenUsage(c)
	require.EqualValues(t, 9, *usage.InputTokens)
	require.EqualValues(t, 4, *usage.OutputTokens)
	require.Nil(t, usage.TotalTokens)
}
