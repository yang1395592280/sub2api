package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountTestTraceProtocolIDsAndUnchangedBody(t *testing.T) {
	tests := []struct {
		name, body, id string
		sse            bool
	}{
		{"chat", `data: {"object":"chat.completion.chunk","id":"chat-1","choices":[{"delta":{"content":"OK"}}]}` + "\n", "chat-1", true},
		{"responses", `data: {"type":"response.created","response":{"id":"resp-1"}}` + "\n" + `data: {"type":"response.output_text.delta","delta":"OK"}` + "\n", "resp-1", true},
		{"anthropic", `data: {"type":"message_start","message":{"id":"msg-1"}}` + "\n" + `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"OK"}}` + "\n", "msg-1", true},
		{"gemini", `{"responseId":"gem-1","candidates":[{"content":{"parts":[{"text":"OK"}]}}]}`, "gem-1", false},
		{"error_not_message", `{"id":"not-a-message","error":{"message":"bad","id":"err-1"}}`, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := &accountTestTrace{started: time.Now(), HeaderName: "x-provider-trace"}
			req, _ := http.NewRequestWithContext(withAccountTestTrace(context.Background(), trace), "POST", "https://example.invalid", nil)
			response := &http.Response{Header: http.Header{"X-Provider-Trace": []string{"real-request"}}, Body: io.NopCloser(strings.NewReader(tt.body))}
			if tt.sse {
				response.Header.Set("Content-Type", "text/event-stream")
			}
			observeAccountTestResponse(req, response)
			// Read in tiny chunks to exercise IDs split across transport reads.
			var out strings.Builder
			buf := make([]byte, 7)
			for {
				n, e := response.Body.Read(buf)
				out.Write(buf[:n])
				if e != nil {
					require.ErrorIs(t, e, io.EOF)
					break
				}
			}
			require.Equal(t, tt.body, out.String())
			require.Equal(t, tt.id, trace.MessageID)
			require.Equal(t, "real-request", trace.RequestID)
			if tt.id != "" {
				require.NotNil(t, trace.TTFTMs)
			} else {
				require.Nil(t, trace.TTFTMs)
			}
		})
	}
}
func TestAccountTestTraceDoesNotObserveNormalRequests(t *testing.T) {
	req, _ := http.NewRequest("POST", "https://example.invalid", nil)
	body := io.NopCloser(strings.NewReader("OK"))
	response := &http.Response{Body: body}
	observeAccountTestResponse(req, response)
	require.Equal(t, body, response.Body)
}

func TestAccountTestMonitorCaptureCancelsOversizedOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &accountTestCaptureWriter{ResponseRecorder: httptest.NewRecorder(), limit: 4, cancel: cancel}
	n, err := w.WriteString("OK")
	require.NoError(t, err)
	require.Equal(t, 2, n)
	_, err = w.WriteString("too large")
	require.Error(t, err)
	require.True(t, w.exceeded)
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.Equal(t, "OK", w.Body.String())
}
