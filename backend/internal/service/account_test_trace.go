package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type accountTestTraceKey struct{}

// accountTestTrace observes only explicitly traced background tests. It never
// retains headers, credentials or the full upstream response.
type accountTestTrace struct {
	mu         sync.Mutex
	started    time.Time
	MessageID  string
	RequestID  string
	HeaderName string
	TTFTMs     *int64
}

type tracedAccountTestUpstream struct{ HTTPUpstream }

func (u *tracedAccountTestUpstream) Do(req *http.Request, proxy string, id int64, concurrency int) (*http.Response, error) {
	response, err := u.HTTPUpstream.Do(req, proxy, id, concurrency)
	observeAccountTestResponse(req, response)
	return response, err
}
func (u *tracedAccountTestUpstream) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	response, err := u.HTTPUpstream.DoWithTLS(req, proxy, id, concurrency, profile)
	observeAccountTestResponse(req, response)
	return response, err
}
func observeAccountTestResponse(req *http.Request, response *http.Response) {
	trace, _ := req.Context().Value(accountTestTraceKey{}).(*accountTestTrace)
	if trace == nil || response == nil {
		return
	}
	trace.mu.Lock()
	names := []string{trace.HeaderName, "x-request-id", "request-id", "x-goog-request-id"}
	for _, name := range names {
		if name != "" && response.Header.Get(name) != "" {
			trace.RequestID = boundedTraceID(response.Header.Get(name))
			break
		}
	}
	trace.mu.Unlock()
	if response.Body != nil {
		response.Body = &accountTestTraceBody{ReadCloser: response.Body, trace: trace, sse: strings.Contains(response.Header.Get("Content-Type"), "text/event-stream")}
	}
}
func boundedTraceID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) > 512 {
		return ""
	}
	for _, r := range id {
		if r < 32 || r == 127 {
			return ""
		}
	}
	return id
}

type accountTestTraceBody struct {
	io.ReadCloser
	trace    *accountTestTrace
	sse      bool
	pending  []byte
	overflow bool
}

func (b *accountTestTraceBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	// Bound both long SSE lines and non-streaming JSON without modifying the
	// bytes seen by the existing account protocol parsers.
	if !b.overflow {
		b.pending = append(b.pending, p[:n]...)
		if b.sse {
			for {
				i := bytes.IndexByte(b.pending, '\n')
				if i < 0 {
					break
				}
				line := bytes.TrimSpace(b.pending[:i])
				b.observe(bytes.TrimPrefix(line, []byte("data:")))
				b.pending = b.pending[i+1:]
			}
		}
		if len(b.pending) > 1<<20 {
			b.pending = nil
			b.overflow = true
		}
		if err != nil {
			b.observe(b.pending)
			b.pending = nil
		}
	}
	return n, err
}
func (b *accountTestTraceBody) observe(data []byte) {
	var v map[string]any
	if json.Unmarshal(bytes.TrimSpace(data), &v) != nil {
		return
	}
	b.trace.mu.Lock()
	defer b.trace.mu.Unlock()
	str := func(m map[string]any, k string) string { s, _ := m[k].(string); return s }
	// IDs are accepted only from known protocol envelopes, never arbitrary id
	// properties in tool calls, content blocks or errors.
	object := str(v, "object")
	id := ""
	if object == "chat.completion" || object == "chat.completion.chunk" || object == "response" || str(v, "type") == "message" {
		id = str(v, "id")
	}
	if m, ok := v["message"].(map[string]any); ok && str(v, "type") == "message_start" {
		id = str(m, "id")
	}
	if m, ok := v["response"].(map[string]any); ok && strings.HasPrefix(str(v, "type"), "response.") {
		id = str(m, "id")
	}
	if _, ok := v["candidates"]; ok {
		id = str(v, "responseId")
	}
	if id = boundedTraceID(id); id != "" {
		b.trace.MessageID = id
	}
	content := false
	if delta, ok := v["delta"].(map[string]any); ok {
		content = str(delta, "text") != ""
	}
	if str(v, "type") == "response.output_text.delta" {
		content = str(v, "delta") != ""
	}
	if choices, ok := v["choices"].([]any); ok {
		for _, c := range choices {
			if m, ok := c.(map[string]any); ok {
				for _, k := range []string{"delta", "message"} {
					if d, ok := m[k].(map[string]any); ok && str(d, "content") != "" {
						content = true
					}
				}
			}
		}
	}
	if candidates, ok := v["candidates"].([]any); ok {
		for _, candidate := range candidates {
			if m, ok := candidate.(map[string]any); ok {
				if c, ok := m["content"].(map[string]any); ok {
					if parts, ok := c["parts"].([]any); ok {
						for _, part := range parts {
							if p, ok := part.(map[string]any); ok && str(p, "text") != "" {
								content = true
							}
						}
					}
				}
			}
		}
	}
	if blocks, ok := v["content"].([]any); ok && len(blocks) > 0 {
		content = true
	}
	if content && b.trace.TTFTMs == nil {
		ms := time.Since(b.trace.started).Milliseconds()
		b.trace.TTFTMs = &ms
	}
}

func withAccountTestTrace(ctx context.Context, trace *accountTestTrace) context.Context {
	return context.WithValue(ctx, accountTestTraceKey{}, trace)
}

// The scheduled monitor has a bounded capture even if an upstream streams
// indefinitely. Canceling also stops the underlying HTTP request.
type accountTestCaptureWriter struct {
	*httptest.ResponseRecorder
	limit    int
	cancel   context.CancelFunc
	exceeded bool
}

func (w *accountTestCaptureWriter) Write(p []byte) (int, error) {
	if w.limit > 0 && w.Body.Len()+len(p) > w.limit {
		w.exceeded = true
		w.cancel()
		return 0, errors.New("monitor output limit exceeded")
	}
	return w.ResponseRecorder.Write(p)
}

func (w *accountTestCaptureWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}
