package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type blockingOpenAIAutoSchedulerOutcomeSink struct {
	started chan struct{}
	release chan struct{}
}

func newBlockingOpenAIAutoSchedulerOutcomeSink() *blockingOpenAIAutoSchedulerOutcomeSink {
	return &blockingOpenAIAutoSchedulerOutcomeSink{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
}

func (s *blockingOpenAIAutoSchedulerOutcomeSink) Record(ctx context.Context, _ OpenAIAutoSchedulerRecordInput) error {
	select {
	case s.started <- struct{}{}:
	default:
	}
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type collectingOpenAIAutoSchedulerOutcomeSink struct {
	mu      sync.Mutex
	records []OpenAIAutoSchedulerRecordInput
	failAt  map[int]error
}

func (s *collectingOpenAIAutoSchedulerOutcomeSink) Record(_ context.Context, input OpenAIAutoSchedulerRecordInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	call := len(s.records) + 1
	s.records = append(s.records, input)
	return s.failAt[call]
}

func (s *collectingOpenAIAutoSchedulerOutcomeSink) snapshot() []OpenAIAutoSchedulerRecordInput {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]OpenAIAutoSchedulerRecordInput(nil), s.records...)
}

func TestOpenAIAutoSchedulerOutcomeRecorderDoesNotBlockWhenFull(t *testing.T) {
	sink := newBlockingOpenAIAutoSchedulerOutcomeSink()
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 1, 1)
	t.Cleanup(func() {
		close(sink.release)
		_ = recorder.Stop(context.Background())
	})

	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 1}))
	select {
	case <-sink.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start processing the first record")
	}
	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 2}))

	start := time.Now()
	require.False(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 3}))
	require.Less(t, time.Since(start), 50*time.Millisecond)

	metrics := recorder.SnapshotMetrics()
	require.EqualValues(t, 2, metrics.Accepted)
	require.EqualValues(t, 1, metrics.Dropped)
}

func TestOpenAIAutoSchedulerOutcomeRecorderStopDrainsAcceptedRecords(t *testing.T) {
	sink := &collectingOpenAIAutoSchedulerOutcomeSink{}
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 4, 1)

	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 1}))
	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 2}))
	require.NoError(t, recorder.Stop(context.Background()))

	require.Equal(t, []OpenAIAutoSchedulerRecordInput{
		{AccountID: 1},
		{AccountID: 2},
	}, sink.snapshot())
	require.EqualValues(t, 2, recorder.SnapshotMetrics().Accepted)
}

func TestOpenAIAutoSchedulerOutcomeRecorderRejectsAfterStop(t *testing.T) {
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(&collectingOpenAIAutoSchedulerOutcomeSink{}, 1, 1)
	require.NoError(t, recorder.Stop(context.Background()))

	require.False(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 1}))
	require.EqualValues(t, 1, recorder.SnapshotMetrics().Dropped)
}

func TestOpenAIAutoSchedulerOutcomeRecorderIsolatesSinkErrors(t *testing.T) {
	sink := &collectingOpenAIAutoSchedulerOutcomeSink{failAt: map[int]error{1: errors.New("write failed")}}
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 2, 1)

	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 1}))
	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 2}))
	require.NoError(t, recorder.Stop(context.Background()))

	require.Len(t, sink.snapshot(), 2)
	metrics := recorder.SnapshotMetrics()
	require.EqualValues(t, 2, metrics.Accepted)
	require.EqualValues(t, 1, metrics.Failed)
}

func TestOpenAIAutoSchedulerOutcomeRecorderClassifiesSuccessfulFeedbackWithConfiguredThresholds(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.SlowThresholdMS = 100
	settings.SevereSlowThresholdMS = 200
	svc := NewOpenAIAutoSchedulerService(nil, fakeOpenAIAutoSchedulerSettingsProvider{settings: settings})

	tests := []struct {
		name      string
		ttfbMS    int
		wantEvent string
	}{
		{name: "success", ttfbMS: 99, wantEvent: OpenAIAutoSchedulerEventSuccess},
		{name: "slow", ttfbMS: 100, wantEvent: OpenAIAutoSchedulerEventSlow},
		{name: "severe slow", ttfbMS: 200, wantEvent: OpenAIAutoSchedulerEventSevereSlow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventSuccess, TtfbMS: &tt.ttfbMS}
			classified := classifyOpenAIAutoSchedulerProductionOutcome(context.Background(), svc, input)
			require.Equal(t, tt.wantEvent, classified.EventType)
		})
	}
}

func TestOpenAIAutoSchedulerOutcomeRecorderRecordsNativeHTTPOutcome(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantEvent  string
		wantError  bool
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       `{"id":"resp_ok","object":"response","model":"gpt-5.4","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`,
			wantEvent:  OpenAIAutoSchedulerEventSuccess,
		},
		{
			name:       "rate limited",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":{"type":"rate_limit_error","message":"slow down"}}`,
			wantEvent:  OpenAIAutoSchedulerEventRateLimited,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			groupID := int64(42)
			requestBody := []byte(`{"model":"gpt-5.4","stream":false,"instructions":"test","input":"hello"}`)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))
			c.Set("api_key", &APIKey{GroupID: &groupID})

			sink := &collectingOpenAIAutoSchedulerOutcomeSink{}
			recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 4, 1)
			svc := &OpenAIGatewayService{
				cfg: &config.Config{},
				httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
					StatusCode: tt.statusCode,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				}},
				openAIAutoSchedulerOutcomeRecorder: recorder,
			}
			account := &Account{
				ID:          9,
				Name:        "scheduler-test",
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Concurrency: 1,
				Credentials: map[string]any{"access_token": "test-token", "chatgpt_account_id": "test-account"},
				Status:      StatusActive,
				Schedulable: true,
			}

			_, err := svc.Forward(context.Background(), c, account, requestBody)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, recorder.Stop(context.Background()))

			records := sink.snapshot()
			require.Len(t, records, 1)
			require.Equal(t, int64(9), records[0].AccountID)
			require.Equal(t, groupID, records[0].GroupID)
			require.Equal(t, "gpt-5.4", records[0].Model)
			require.Equal(t, tt.wantEvent, records[0].EventType)
			require.Equal(t, tt.statusCode, *records[0].StatusCode)
			require.NotNil(t, records[0].LatencyMS)
		})
	}
}

func TestOpenAIAutoSchedulerOutcomeRecorderRecordsPassthroughHTTPOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(88)
	requestBody := []byte(`{"model":"gpt-5.4","stream":false,"instructions":"test","input":"hello"}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))
	c.Set("api_key", &APIKey{GroupID: &groupID})

	sink := &collectingOpenAIAutoSchedulerOutcomeSink{}
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 4, 1)
	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
		httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_ok","object":"response","model":"gpt-5.4","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`)),
		}},
		openAIAutoSchedulerOutcomeRecorder: recorder,
	}
	account := &Account{
		ID:          11,
		Name:        "passthrough-test",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "test-token"},
		Status:      StatusActive,
		Schedulable: true,
	}

	_, err := svc.forwardOpenAIPassthrough(context.Background(), c, account, requestBody, "gpt-5.4", nil, false, time.Now())
	require.NoError(t, err)
	require.NoError(t, recorder.Stop(context.Background()))

	records := sink.snapshot()
	require.Len(t, records, 1)
	require.Equal(t, OpenAIAutoSchedulerEventSuccess, records[0].EventType)
	require.Equal(t, int64(11), records[0].AccountID)
	require.Equal(t, groupID, records[0].GroupID)
	require.Equal(t, "gpt-5.4", records[0].Model)
}
