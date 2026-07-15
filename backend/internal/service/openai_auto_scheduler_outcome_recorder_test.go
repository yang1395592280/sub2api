package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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

type blockingOutcomeRecorderLogHandler struct {
	release <-chan struct{}
}

func (h *blockingOutcomeRecorderLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *blockingOutcomeRecorderLogHandler) Handle(_ context.Context, record slog.Record) error {
	if record.Message == "OpenAI auto scheduler outcome recorder dropped feedback" {
		<-h.release
	}
	return nil
}

func (h *blockingOutcomeRecorderLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *blockingOutcomeRecorderLogHandler) WithGroup(string) slog.Handler      { return h }

type collectingOutcomeRecorderLogHandler struct {
	mu       sync.Mutex
	messages []string
}

func (h *collectingOutcomeRecorderLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *collectingOutcomeRecorderLogHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, record.Message)
	return nil
}

func (h *collectingOutcomeRecorderLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *collectingOutcomeRecorderLogHandler) WithGroup(string) slog.Handler      { return h }

func (h *collectingOutcomeRecorderLogHandler) count(message string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	count := 0
	for _, recorded := range h.messages {
		if recorded == message {
			count++
		}
	}
	return count
}

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

func TestOpenAIAutoSchedulerOutcomeRecorderTryRecordDoesNotWaitForStopLock(t *testing.T) {
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(&collectingOpenAIAutoSchedulerOutcomeSink{}, 1, 1)
	recorder.queueMu.Lock()

	resultCh := make(chan bool, 1)
	start := time.Now()
	go func() {
		resultCh <- recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 1})
	}()

	select {
	case accepted := <-resultCh:
		require.False(t, accepted)
		require.Less(t, time.Since(start), 50*time.Millisecond)
	case <-time.After(50 * time.Millisecond):
		recorder.queueMu.Unlock()
		<-resultCh
		_ = recorder.Stop(context.Background())
		t.Fatal("TryRecord blocked waiting for the lifecycle lock")
	}
	recorder.queueMu.Unlock()
	require.NoError(t, recorder.Stop(context.Background()))
	require.EqualValues(t, 1, recorder.SnapshotMetrics().Dropped)
}

func TestOpenAIAutoSchedulerOutcomeRecorderTryRecordDoesNotRunDropLogger(t *testing.T) {
	sink := newBlockingOpenAIAutoSchedulerOutcomeSink()
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 1, 1)
	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 1}))
	select {
	case <-sink.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start processing the first record")
	}
	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 2}))

	releaseLog := make(chan struct{})
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(&blockingOutcomeRecorderLogHandler{release: releaseLog}))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	resultCh := make(chan bool, 1)
	go func() {
		resultCh <- recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 3})
	}()
	select {
	case accepted := <-resultCh:
		require.False(t, accepted)
	case <-time.After(50 * time.Millisecond):
		close(releaseLog)
		<-resultCh
		close(sink.release)
		_ = recorder.Stop(context.Background())
		t.Fatal("TryRecord synchronously invoked the drop logger")
	}
	close(releaseLog)
	close(sink.release)
	require.NoError(t, recorder.Stop(context.Background()))
}

func TestOpenAIAutoSchedulerOutcomeRecorderLogsEachDroppedCountOnce(t *testing.T) {
	handler := &collectingOutcomeRecorderLogHandler{}
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	recorder := &OpenAIAutoSchedulerOutcomeRecorder{}
	recorder.recordDropped()
	recorder.logDroppedFeedback()
	recorder.logDroppedFeedback()

	require.Equal(t, 1, handler.count("OpenAI auto scheduler outcome recorder dropped feedback"))
}

func TestOpenAIAutoSchedulerOutcomeRecorderLogsNonPowerOfTwoDroppedTotal(t *testing.T) {
	handler := &collectingOutcomeRecorderLogHandler{}
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	recorder := &OpenAIAutoSchedulerOutcomeRecorder{}
	recorder.recordDropped()
	recorder.recordDropped()
	recorder.recordDropped()
	recorder.logDroppedFeedback()

	require.Equal(t, 1, handler.count("OpenAI auto scheduler outcome recorder dropped feedback"))
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

func TestOpenAIAutoSchedulerOutcomeRecorderConcurrentTryRecordAndStop(t *testing.T) {
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(&collectingOpenAIAutoSchedulerOutcomeSink{}, 32, 2)
	start := make(chan struct{})
	var writers sync.WaitGroup
	for i := 0; i < 16; i++ {
		writers.Add(1)
		go func(accountID int64) {
			defer writers.Done()
			<-start
			for recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: accountID}) {
			}
		}(int64(i + 1))
	}

	close(start)
	require.NoError(t, recorder.Stop(context.Background()))
	writers.Wait()
	require.False(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 99}))
}

func TestOpenAIAutoSchedulerOutcomeRecorderConcurrentStopIsIdempotent(t *testing.T) {
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(&collectingOpenAIAutoSchedulerOutcomeSink{}, 4, 1)
	var stops sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < cap(errs); i++ {
		stops.Add(1)
		go func() {
			defer stops.Done()
			errs <- recorder.Stop(context.Background())
		}()
	}
	stops.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func TestOpenAIAutoSchedulerOutcomeRecorderStopCanWaitAgainAfterTimeout(t *testing.T) {
	sink := newBlockingOpenAIAutoSchedulerOutcomeSink()
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 1, 1)
	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 1}))
	select {
	case <-sink.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start processing the record")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, recorder.Stop(stopCtx), context.DeadlineExceeded)
	close(sink.release)
	require.NoError(t, recorder.Stop(context.Background()))
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

func TestOpenAIAutoSchedulerOutcomeRecorderUsesStrictProductionPersistence(t *testing.T) {
	settings := enabledOpenAIAutoSchedulerSettings()
	repo := &fakeOpenAIAutoSchedulerRepo{
		groups: map[int64]Group{2: {
			ID: 2, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true,
		}},
		err: errors.New("repository unavailable"),
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: settings})
	input := OpenAIAutoSchedulerRecordInput{
		AccountID: 1, GroupID: 2, Model: "gpt-5", EventType: OpenAIAutoSchedulerEventSuccess,
	}

	// Existing callers retain best-effort behavior.
	require.NoError(t, svc.Record(context.Background(), input))
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(svc, 1, 1)
	require.True(t, recorder.TryRecord(input))
	require.Eventually(t, func() bool {
		return recorder.SnapshotMetrics().Failed == 1
	}, time.Second, time.Millisecond)
	require.NoError(t, recorder.Stop(context.Background()))
}

func TestOpenAIAutoSchedulerOutcomeRecorderFeatureOffIsNoop(t *testing.T) {
	tests := []struct {
		name     string
		settings OpenAIAutoSchedulerSettings
		group    Group
	}{
		{
			name: "global disabled",
			settings: func() OpenAIAutoSchedulerSettings {
				settings := enabledOpenAIAutoSchedulerSettings()
				settings.Enabled = false
				return settings
			}(),
			group: Group{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true},
		},
		{name: "inactive group", settings: enabledOpenAIAutoSchedulerSettings(), group: Group{ID: 2, Platform: PlatformOpenAI, Status: StatusDisabled, OpenAIAutoSchedulerEnabled: true}},
		{name: "scheduler disabled group", settings: enabledOpenAIAutoSchedulerSettings(), group: Group{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: false}},
		{name: "non openai group", settings: enabledOpenAIAutoSchedulerSettings(), group: Group{ID: 2, Platform: PlatformAnthropic, Status: StatusActive, OpenAIAutoSchedulerEnabled: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeOpenAIAutoSchedulerRepo{groups: map[int64]Group{2: tt.group}}
			svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: tt.settings})
			recorder := NewOpenAIAutoSchedulerOutcomeRecorder(svc, 1, 1)
			require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{
				AccountID: 1, GroupID: 2, Model: "gpt-5", EventType: OpenAIAutoSchedulerEventSuccess,
			}))
			require.NoError(t, recorder.Stop(context.Background()))

			metrics := recorder.SnapshotMetrics()
			require.Zero(t, metrics.Failed)
			require.Zero(t, repo.getStateCalls)
			require.Empty(t, repo.events)
		})
	}
}

func TestOpenAIAutoSchedulerOutcomeRecorderExposesRuntimeQueueDepth(t *testing.T) {
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
		t.Fatal("worker did not start")
	}
	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 2}))
	require.False(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 3}))

	metrics := recorder.SnapshotMetrics()
	require.Equal(t, 1, metrics.QueueDepth)
	require.EqualValues(t, 1, metrics.Dropped)
}

func TestOpenAIAutoSchedulerOutcomeRecorderPeriodicallyLogsRuntimeMetrics(t *testing.T) {
	handler := &collectingOutcomeRecorderLogHandler{}
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	recorder := newOpenAIAutoSchedulerOutcomeRecorder(&collectingOpenAIAutoSchedulerOutcomeSink{}, 1, 1, time.Millisecond)
	require.Eventually(t, func() bool {
		return handler.count("OpenAI auto scheduler outcome recorder metrics") > 0
	}, time.Second, time.Millisecond)
	require.NoError(t, recorder.Stop(context.Background()))
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

	_, err := svc.forwardOpenAIPassthrough(context.Background(), c, account, requestBody, requestBody, "gpt-5.4", false, nil, false, time.Now())
	require.NoError(t, err)
	require.NoError(t, recorder.Stop(context.Background()))

	records := sink.snapshot()
	require.Len(t, records, 1)
	require.Equal(t, OpenAIAutoSchedulerEventSuccess, records[0].EventType)
	require.Equal(t, int64(11), records[0].AccountID)
	require.Equal(t, groupID, records[0].GroupID)
	require.Equal(t, "gpt-5.4", records[0].Model)
}

func TestOpenAIAutoSchedulerOutcomeRecorderCoversAdditionalTransportErrors(t *testing.T) {
	newConfig := func() *config.Config {
		cfg := &config.Config{}
		cfg.Security.URLAllowlist.Enabled = false
		cfg.Security.URLAllowlist.AllowInsecureHTTP = true
		return cfg
	}
	newAPIKeyAccount := func() *Account {
		return &Account{
			ID: 90, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
			Credentials: map[string]any{"api_key": "sk-test", "base_url": "http://upstream.example"},
		}
	}
	newContext := func(t *testing.T, path string, body []byte) (*gin.Context, int64) {
		t.Helper()
		groupID := int64(91)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("api_key", &APIKey{GroupID: &groupID})
		return c, groupID
	}
	assertOutcome := func(t *testing.T, sink *collectingOpenAIAutoSchedulerOutcomeSink, recorder *OpenAIAutoSchedulerOutcomeRecorder, want string) {
		t.Helper()
		require.NoError(t, recorder.Stop(context.Background()))
		records := sink.snapshot()
		require.Len(t, records, 1)
		require.Equal(t, want, records[0].EventType)
	}

	t.Run("embeddings 429", func(t *testing.T) {
		body := []byte(`{"model":"text-embedding-3-small","input":"hello"}`)
		c, _ := newContext(t, "/v1/embeddings", body)
		sink := &collectingOpenAIAutoSchedulerOutcomeSink{}
		recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 2, 1)
		svc := &OpenAIGatewayService{cfg: newConfig(), httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"rate_limit_error","message":"slow down"}}`)),
		}}, openAIAutoSchedulerOutcomeRecorder: recorder}
		account := newAPIKeyAccount()
		_, err := svc.ForwardEmbeddings(context.Background(), c, account, body, "")
		require.Error(t, err)
		assertOutcome(t, sink, recorder, OpenAIAutoSchedulerEventRateLimited)
	})

	for _, transport := range []struct {
		name string
		path string
		run  func(*OpenAIGatewayService, *gin.Context) error
	}{
		{
			name: "raw chat transport error",
			path: "/v1/chat/completions",
			run: func(svc *OpenAIGatewayService, c *gin.Context) error {
				body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
				_, err := svc.forwardAsRawChatCompletions(context.Background(), c, newAPIKeyAccount(), body, "")
				return err
			},
		},
		{
			name: "normal chat transport error",
			path: "/v1/chat/completions",
			run: func(svc *OpenAIGatewayService, c *gin.Context) error {
				body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
				account := &Account{ID: 92, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "test", "chatgpt_account_id": "acct"}}
				_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
				return err
			},
		},
		{
			name: "images transport error",
			path: "/v1/images/generations",
			run: func(svc *OpenAIGatewayService, c *gin.Context) error {
				body := []byte(`{"model":"gpt-image-2","prompt":"draw"}`)
				parsed, err := svc.ParseOpenAIImagesRequest(c, body)
				require.NoError(t, err)
				account := newAPIKeyAccount()
				_, err = svc.ForwardImages(context.Background(), c, account, body, parsed, "")
				return err
			},
		},
	} {
		t.Run(transport.name, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.4","messages":[],"prompt":"draw"}`)
			c, _ := newContext(t, transport.path, body)
			sink := &collectingOpenAIAutoSchedulerOutcomeSink{}
			recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 2, 1)
			svc := &OpenAIGatewayService{
				cfg:                                newConfig(),
				httpUpstream:                       &httpUpstreamRecorder{err: errors.New("dial failed")},
				openAIAutoSchedulerOutcomeRecorder: recorder,
			}
			require.Error(t, transport.run(svc, c))
			assertOutcome(t, sink, recorder, OpenAIAutoSchedulerEventError)
		})
	}
}

func TestOpenAIHTTPOutcomeFinalizersIgnoreLocalValidationFailures(t *testing.T) {
	newContext := func(path string, body []byte) *gin.Context {
		groupID := int64(93)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("api_key", &APIKey{GroupID: &groupID})
		return c
	}
	newRecorderService := func() (*OpenAIGatewayService, *OpenAIAutoSchedulerOutcomeRecorder) {
		recorder := NewOpenAIAutoSchedulerOutcomeRecorder(&collectingOpenAIAutoSchedulerOutcomeSink{}, 2, 1)
		cfg := &config.Config{}
		cfg.Security.URLAllowlist.Enabled = false
		cfg.Security.URLAllowlist.AllowInsecureHTTP = true
		return &OpenAIGatewayService{cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, openAIAutoSchedulerOutcomeRecorder: recorder}, recorder
	}
	assertNoOutcome := func(t *testing.T, recorder *OpenAIAutoSchedulerOutcomeRecorder) {
		t.Helper()
		require.NoError(t, recorder.Stop(context.Background()))
		require.Zero(t, recorder.SnapshotMetrics().Accepted)
	}
	configureFastBlock := func(t *testing.T, svc *OpenAIGatewayService) {
		t.Helper()
		settings := &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
			ServiceTier: OpenAIFastTierPriority, Action: BetaPolicyActionBlock, Scope: BetaPolicyScopeAll,
			ErrorMessage: "blocked locally", ModelWhitelist: []string{"gpt-5.4"}, FallbackAction: BetaPolicyActionPass,
		}}}
		raw, err := json.Marshal(settings)
		require.NoError(t, err)
		repo := &openAIFastPolicyRepoStub{values: map[string]string{SettingKeyOpenAIFastPolicySettings: string(raw)}}
		svc.settingService = NewSettingService(repo, svc.cfg)
	}

	t.Run("normal chat fast policy", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"service_tier":"priority"}`)
		svc, recorder := newRecorderService()
		configureFastBlock(t, svc)
		account := &Account{ID: 94, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test"}}
		_, err := svc.ForwardAsChatCompletions(context.Background(), newContext("/v1/chat/completions", body), account, body, "", "")
		require.Error(t, err)
		assertNoOutcome(t, recorder)
	})

	t.Run("raw chat fast policy", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"service_tier":"priority"}`)
		svc, recorder := newRecorderService()
		configureFastBlock(t, svc)
		account := &Account{ID: 95, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test", "base_url": "http://upstream.example"}}
		_, err := svc.forwardAsRawChatCompletions(context.Background(), newContext("/v1/chat/completions", body), account, body, "")
		require.Error(t, err)
		assertNoOutcome(t, recorder)
	})

	t.Run("embeddings invalid URL", func(t *testing.T) {
		body := []byte(`{"model":"text-embedding-3-small","input":"hello"}`)
		svc, recorder := newRecorderService()
		account := &Account{ID: 96, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test", "base_url": "://invalid"}}
		_, err := svc.ForwardEmbeddings(context.Background(), newContext("/v1/embeddings", body), account, body, "")
		require.Error(t, err)
		assertNoOutcome(t, recorder)
	})

	t.Run("images invalid mapped model", func(t *testing.T) {
		body := []byte(`{"model":"gpt-image-2","prompt":"draw"}`)
		svc, recorder := newRecorderService()
		c := newContext("/v1/images/generations", body)
		parsed, err := svc.ParseOpenAIImagesRequest(c, body)
		require.NoError(t, err)
		account := &Account{ID: 97, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"api_key": "test", "base_url": "http://upstream.example", "model_mapping": map[string]any{"gpt-image-2": "gpt-5.4"},
		}}
		_, err = svc.ForwardImages(context.Background(), c, account, body, parsed, "")
		require.Error(t, err)
		assertNoOutcome(t, recorder)
	})
}
