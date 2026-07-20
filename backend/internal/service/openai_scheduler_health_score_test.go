package service

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyOpenAISchedulerHealthEvent(t *testing.T) {
	now := time.Unix(1_000, 0)
	settings := DefaultOpenAISchedulerHealthSettings()
	settings.SlowThresholdMS = 1_000
	settings.SevereSlowThresholdMS = 2_000
	settings.ConsecutiveSlowBreakerThreshold = 2
	settings.ConsecutiveErrorBreakerThreshold = 2
	settings.CooldownSeconds = 60
	settings.HalfOpenSuccessThreshold = 2

	t.Run("fast real success initializes prediction", func(t *testing.T) {
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{}, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSuccess, TTFTMS: 400, OccurredAt: now,
		}, settings)
		require.Equal(t, 400.0, got.PredictedTTFTMS)
		require.Equal(t, int64(1), got.RealSampleCount)
		require.Equal(t, OpenAIAutoSchedulerStateRunning, got.State)
		require.Equal(t, 1, got.ConsecutiveSuccess)
		require.Equal(t, now.Add(30*time.Minute), got.ExpiresAt)
	})

	t.Run("slow real success uses real EWMA and observes", func(t *testing.T) {
		lastRealAt := now.Add(-time.Minute)
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500, RealSampleCount: 4, LastRealAt: &lastRealAt,
		}, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSlow, TTFTMS: 1_500, OccurredAt: now,
		}, settings)
		require.Equal(t, 700.0, got.PredictedTTFTMS)
		require.Equal(t, OpenAIAutoSchedulerStateObserving, got.State)
		require.Equal(t, 1, got.ConsecutiveSlow)
	})

	t.Run("severe slow opens after configured breaker", func(t *testing.T) {
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateObserving, ConsecutiveSlow: 1, RealSampleCount: 1,
		}, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSevereSlow, TTFTMS: 2_500, OccurredAt: now,
		}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateOpen, got.State)
		require.Equal(t, 2, got.ConsecutiveSlow)
		require.Equal(t, now.Add(time.Minute), *got.CooldownUntil)
	})

	t.Run("429 updates error and rate limited rates", func(t *testing.T) {
		status := 429
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{}, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventRateLimited, StatusCode: &status, OccurredAt: now,
		}, settings)
		require.Equal(t, 1.0, got.ErrorRate)
		require.Equal(t, 1.0, got.RateLimitedRate)
		require.Zero(t, got.ServerErrorRate)
		require.Equal(t, 1, got.ConsecutiveError)
	})

	t.Run("5xx updates server error rate and opens breaker", func(t *testing.T) {
		status := 503
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateObserving, ConsecutiveError: 1, RealSampleCount: 1,
		}, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventError, StatusCode: &status, OccurredAt: now,
		}, settings)
		require.Equal(t, 0.2, got.ServerErrorRate)
		require.Equal(t, OpenAIAutoSchedulerStateOpen, got.State)
	})

	t.Run("request error keeps account health unchanged", func(t *testing.T) {
		status := 400
		current := OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500,
			RealSampleCount: 4, ErrorRate: 0.25, ConsecutiveError: 1,
		}
		got := ApplyOpenAISchedulerHealthEvent(now, current, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventRequestError, StatusCode: &status, OccurredAt: now,
		}, settings)
		require.Equal(t, current.State, got.State)
		require.Equal(t, current.RealSampleCount, got.RealSampleCount)
		require.Equal(t, current.ErrorRate, got.ErrorRate)
		require.Equal(t, current.ConsecutiveError, got.ConsecutiveError)
		require.Equal(t, now.Add(settings.StateTTL), got.ExpiresAt)
	})

	t.Run("first TTFT initializes prediction after an error-only sample", func(t *testing.T) {
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateObserving, RealSampleCount: 1, ErrorRate: 1,
		}, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSuccess, TTFTMS: 500, OccurredAt: now,
		}, settings)
		require.Equal(t, 500.0, got.PredictedTTFTMS)
	})

	t.Run("probe cannot overwrite fresh real sample", func(t *testing.T) {
		lastRealAt := now.Add(-time.Minute)
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			PredictedTTFTMS: 1_400, RealSampleCount: 20, LastRealAt: &lastRealAt,
		}, OpenAISchedulerHealthEvent{Source: HealthSourceProbe, EventType: OpenAIAutoSchedulerEventSevereSlow, TTFTMS: 12_000, OccurredAt: now}, DefaultOpenAISchedulerHealthSettings())
		require.Equal(t, 1_400.0, got.PredictedTTFTMS)
		require.Equal(t, int64(1), got.ProbeSampleCount)
	})

	t.Run("expired snapshot resets before applying event", func(t *testing.T) {
		expired := now.Add(-time.Second)
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateOpen, PredictedTTFTMS: 9_000, RealSampleCount: 10,
			ConsecutiveError: 4, ExpiresAt: expired,
		}, OpenAISchedulerHealthEvent{Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSuccess, TTFTMS: 300, OccurredAt: now}, settings)
		require.Equal(t, 300.0, got.PredictedTTFTMS)
		require.Equal(t, int64(1), got.RealSampleCount)
		require.Zero(t, got.ConsecutiveError)
		require.Equal(t, OpenAIAutoSchedulerStateRunning, got.State)
	})

	t.Run("open circuit remains open before cooldown", func(t *testing.T) {
		cooldown := now.Add(time.Second)
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateOpen, CooldownUntil: &cooldown, RealSampleCount: 1,
		}, OpenAISchedulerHealthEvent{Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSuccess, TTFTMS: 300, OccurredAt: now}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateOpen, got.State)
	})

	t.Run("error-open circuit stays open with original cooldown after slow sample", func(t *testing.T) {
		opened := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateObserving, ConsecutiveError: 1, RealSampleCount: 1,
		}, OpenAISchedulerHealthEvent{Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventError, OccurredAt: now}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateOpen, opened.State)
		originalCooldown := *opened.CooldownUntil

		got := ApplyOpenAISchedulerHealthEvent(now.Add(time.Second), opened, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSlow, TTFTMS: 1_500, OccurredAt: now.Add(time.Second),
		}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateOpen, got.State)
		require.Equal(t, originalCooldown, *got.CooldownUntil)
		require.Equal(t, 1, got.ConsecutiveSlow)
		require.Zero(t, got.ConsecutiveError)
	})

	t.Run("unexpired open circuit repeated error preserves original cooldown", func(t *testing.T) {
		opened := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateObserving, ConsecutiveError: 1, RealSampleCount: 1,
		}, OpenAISchedulerHealthEvent{Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventError, OccurredAt: now}, settings)
		originalCooldown := *opened.CooldownUntil

		got := ApplyOpenAISchedulerHealthEvent(now.Add(time.Second), opened, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventError, OccurredAt: now.Add(time.Second),
		}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateOpen, got.State)
		require.Equal(t, originalCooldown, *got.CooldownUntil)
		require.Equal(t, 3, got.ConsecutiveError)
	})

	t.Run("slow-open circuit stays open with original cooldown after error", func(t *testing.T) {
		opened := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateObserving, ConsecutiveSlow: 1, RealSampleCount: 1,
		}, OpenAISchedulerHealthEvent{Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSlow, TTFTMS: 1_500, OccurredAt: now}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateOpen, opened.State)
		originalCooldown := *opened.CooldownUntil

		got := ApplyOpenAISchedulerHealthEvent(now.Add(time.Second), opened, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventError, OccurredAt: now.Add(time.Second),
		}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateOpen, got.State)
		require.Equal(t, originalCooldown, *got.CooldownUntil)
		require.Equal(t, 1, got.ConsecutiveError)
		require.Zero(t, got.ConsecutiveSlow)
	})

	t.Run("half open slow immediately reopens", func(t *testing.T) {
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateHalfOpen, ConsecutiveSuccess: 1, RealSampleCount: 1,
		}, OpenAISchedulerHealthEvent{Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSlow, TTFTMS: 1_500, OccurredAt: now}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateOpen, got.State)
		require.Equal(t, now.Add(time.Minute), *got.CooldownUntil)
		require.Equal(t, 1, got.ConsecutiveSlow)
		require.Zero(t, got.ConsecutiveSuccess)
	})

	t.Run("expired open circuit bad sample refreshes cooldown", func(t *testing.T) {
		expiredCooldown := now.Add(-time.Second)
		got := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateOpen, CooldownUntil: &expiredCooldown, RealSampleCount: 1,
		}, OpenAISchedulerHealthEvent{Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventRateLimited, OccurredAt: now}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateOpen, got.State)
		require.Equal(t, now.Add(time.Minute), *got.CooldownUntil)
		require.Equal(t, 1, got.ConsecutiveError)
	})

	t.Run("cooldown success enters half open and next success recovers", func(t *testing.T) {
		cooldown := now.Add(-time.Second)
		halfOpen := ApplyOpenAISchedulerHealthEvent(now, OpenAISchedulerHealthSnapshot{
			State: OpenAIAutoSchedulerStateOpen, CooldownUntil: &cooldown, RealSampleCount: 1,
		}, OpenAISchedulerHealthEvent{Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSuccess, TTFTMS: 300, OccurredAt: now}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateHalfOpen, halfOpen.State)
		require.Equal(t, 1, halfOpen.ConsecutiveSuccess)

		recovered := ApplyOpenAISchedulerHealthEvent(now.Add(time.Second), halfOpen, OpenAISchedulerHealthEvent{
			Source: HealthSourceReal, EventType: OpenAIAutoSchedulerEventSuccess, TTFTMS: 250, OccurredAt: now.Add(time.Second),
		}, settings)
		require.Equal(t, OpenAIAutoSchedulerStateRunning, recovered.State)
		require.Nil(t, recovered.CooldownUntil)
	})
}

type recordingOpenAISchedulerHealthRepository struct {
	mu       sync.Mutex
	states   map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot
	getDelay time.Duration
	upserts  int
}

func (r *recordingOpenAISchedulerHealthRepository) GetBatch(_ context.Context, keys []OpenAISchedulerHealthKey) (map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, error) {
	time.Sleep(r.getDelay)
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make(map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, len(keys))
	for _, key := range keys {
		if state, ok := r.states[key]; ok {
			result[key] = state
		}
	}
	return result, nil
}

func (r *recordingOpenAISchedulerHealthRepository) Upsert(_ context.Context, snapshot OpenAISchedulerHealthSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.states == nil {
		r.states = map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}
	}
	r.states[snapshot.Key] = snapshot
	r.upserts++
	return nil
}

func (r *recordingOpenAISchedulerHealthRepository) snapshot() map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make(map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, len(r.states))
	for key, state := range r.states {
		result[key] = state
	}
	return result
}

type strictCollectingLegacyOutcomeSink struct {
	mu      sync.Mutex
	records []OpenAIAutoSchedulerRecordInput
}

func (s *strictCollectingLegacyOutcomeSink) Record(ctx context.Context, input OpenAIAutoSchedulerRecordInput) error {
	return s.RecordOutcome(ctx, input)
}

func (s *strictCollectingLegacyOutcomeSink) RecordOutcome(_ context.Context, input OpenAIAutoSchedulerRecordInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, input)
	return nil
}

func (s *strictCollectingLegacyOutcomeSink) snapshot() []OpenAIAutoSchedulerRecordInput {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]OpenAIAutoSchedulerRecordInput(nil), s.records...)
}

func TestOpenAISchedulerHealthOutcomeSink(t *testing.T) {
	repo := &recordingOpenAISchedulerHealthRepository{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}}
	healthSink := NewOpenAISchedulerHealthEventSink(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})
	legacy := &strictCollectingLegacyOutcomeSink{}
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(legacy, 8, 2, healthSink)

	ttft := 500
	inputs := []OpenAIAutoSchedulerRecordInput{
		{AccountID: 7, GroupID: 10, Model: "legacy-a", ModelFamily: " GPT-5.4 ", Endpoint: " RESPONSES ", Transport: " HTTP_SSE ", EventType: OpenAIAutoSchedulerEventSuccess, TtfbMS: &ttft},
		{AccountID: 7, GroupID: 11, Model: "legacy-b", ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "responses_websockets_v2", EventType: OpenAIAutoSchedulerEventSuccess, TtfbMS: &ttft},
		{AccountID: 7, GroupID: 12, Model: "legacy-c", ModelFamily: "gpt-5.4", Endpoint: "chat_completions", Transport: "http_sse", EventType: OpenAIAutoSchedulerEventSuccess, TtfbMS: &ttft},
		{AccountID: 7, GroupID: 13, Model: "legacy-missing", ModelFamily: "", Endpoint: "responses", Transport: "http_sse", EventType: OpenAIAutoSchedulerEventSuccess, TtfbMS: &ttft},
	}
	for _, input := range inputs {
		require.True(t, recorder.TryRecord(input))
	}
	require.NoError(t, recorder.Stop(context.Background()))

	states := repo.snapshot()
	require.Len(t, states, 3, "transport and endpoint dimensions must remain separate")
	require.Contains(t, states, OpenAISchedulerHealthKey{AccountID: 7, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse"})
	require.Contains(t, states, OpenAISchedulerHealthKey{AccountID: 7, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "responses_websockets_v2"})
	require.Contains(t, states, OpenAISchedulerHealthKey{AccountID: 7, ModelFamily: "gpt-5.4", Endpoint: "chat_completions", Transport: "http_sse"})
	require.Len(t, legacy.snapshot(), 4, "missing unified metadata must not suppress the legacy strict write")
}

func TestOpenAISchedulerHealthOutcomeSinkSharesSettingsCacheAcrossRecords(t *testing.T) {
	settingsRepo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{
		SettingKeyOpenAIAutoSchedulerSettings: `{"mode":"balanced","shadow_mode":false,"health_ttl_seconds":1800,"real_sample_fresh_seconds":300}`,
	}}
	settingsService := NewSettingService(settingsRepo, &config.Config{})
	healthRepo := &recordingOpenAISchedulerHealthRepository{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}}
	healthSink := NewOpenAISchedulerHealthEventSink(healthRepo, settingsService)
	input := OpenAIAutoSchedulerRecordInput{
		AccountID: 7, GroupID: 10, Model: "gpt-5.4", ModelFamily: "gpt-5.4",
		Endpoint: "responses", Transport: OpenAIUpstreamTransportHTTPSSE,
		EventType: OpenAIAutoSchedulerEventSuccess,
	}

	require.NoError(t, healthSink.Record(context.Background(), input))
	require.NoError(t, healthSink.Record(context.Background(), input))
	require.Equal(t, 1, settingsRepo.calls())
}

func TestOpenAIAutoSchedulerSettingsCacheIsSharedBySelectionAndOutcome(t *testing.T) {
	settingsRepo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{
		SettingKeyOpenAIAutoSchedulerSettings: `{"mode":"balanced","shadow_mode":false,"health_ttl_seconds":1800,"real_sample_fresh_seconds":300}`,
	}}
	settingsService := NewSettingService(settingsRepo, &config.Config{})
	gateway := &OpenAIGatewayService{settingService: settingsService}
	scheduler := &defaultOpenAIAccountScheduler{service: gateway}
	_ = scheduler.withOpenAIBalancedRuntimeSettings(context.Background(), OpenAIAccountScheduleRequest{})

	healthRepo := &recordingOpenAISchedulerHealthRepository{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}}
	healthSink := NewOpenAISchedulerHealthEventSink(healthRepo, settingsService)
	require.NoError(t, healthSink.Record(context.Background(), OpenAIAutoSchedulerRecordInput{
		AccountID: 8, GroupID: 10, Model: "gpt-5.4", ModelFamily: "gpt-5.4",
		Endpoint: "responses", Transport: OpenAIUpstreamTransportHTTPSSE,
		EventType: OpenAIAutoSchedulerEventSuccess,
	}))

	require.Equal(t, 1, settingsRepo.calls())
}

func TestOpenAISchedulerHealthOutcomeSinkSerializesSameKeyAcrossTwoWorkers(t *testing.T) {
	repo := &recordingOpenAISchedulerHealthRepository{
		states:   map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{},
		getDelay: 20 * time.Millisecond,
	}
	healthSink := NewOpenAISchedulerHealthEventSink(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(&strictCollectingLegacyOutcomeSink{}, 2, 2, healthSink)
	ttft := 500
	input := OpenAIAutoSchedulerRecordInput{
		AccountID: 9, GroupID: 10, Model: "gpt-5.4", ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse",
		EventType: OpenAIAutoSchedulerEventSuccess, TtfbMS: &ttft,
	}
	require.True(t, recorder.TryRecord(input))
	require.True(t, recorder.TryRecord(input))
	require.NoError(t, recorder.Stop(context.Background()))

	states := repo.snapshot()
	require.Len(t, states, 1)
	for _, state := range states {
		require.Equal(t, int64(2), state.RealSampleCount, "load-apply-upsert lost an accepted sample")
	}
}

func TestOpenAISchedulerHealthAttemptMetadata(t *testing.T) {
	t.Run("success uses final upstream model endpoint and actual transport", func(t *testing.T) {
		got := openAIAutoSchedulerHealthMetadataForAttempt(
			openAIAutoSchedulerAttemptMetadata{ModelFamily: " pre-fallback ", Endpoint: "responses", Transport: OpenAIUpstreamTransportResponsesWebsocketV2Ingress},
			&OpenAIForwardResult{UpstreamModel: " Final-Model ", UpstreamEndpoint: "/v1/chat/completions", OpenAIWSMode: true},
		)
		require.Equal(t, openAIAutoSchedulerAttemptMetadata{
			ModelFamily: "final-model", Endpoint: "chat_completions", Transport: OpenAIUpstreamTransportResponsesWebsocketV2,
		}, got)
	})

	t.Run("failure keeps metadata fixed before attempt", func(t *testing.T) {
		got := openAIAutoSchedulerHealthMetadataForAttempt(
			openAIAutoSchedulerAttemptMetadata{ModelFamily: " Mapped-Model ", Endpoint: "embeddings", Transport: OpenAIUpstreamTransportHTTPSSE},
			nil,
		)
		require.Equal(t, openAIAutoSchedulerAttemptMetadata{
			ModelFamily: "mapped-model", Endpoint: "embeddings", Transport: OpenAIUpstreamTransportHTTPSSE,
		}, got)
	})
}

func TestOpenAISchedulerHealthForwardAttemptCarriesFixedAndFinalMetadata(t *testing.T) {
	legacy := &strictCollectingLegacyOutcomeSink{}
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(legacy, 2, 1)
	svc := &OpenAIGatewayService{openAIAutoSchedulerOutcomeRecorder: recorder}
	groupID := int64(10)
	account := &Account{ID: 8, Platform: PlatformOpenAI}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("api_key", &APIKey{GroupID: &groupID})
	ctx, attempt := beginOpenAIUpstreamAttempt(context.Background())
	armOpenAIUpstreamAttempt(ctx, openAIAutoSchedulerAttemptMetadata{
		ModelFamily: " mapped-before-attempt ", Endpoint: openAISchedulerHealthEndpointResponses,
		Transport: OpenAIUpstreamTransportHTTPSSE,
	})
	svc.recordOpenAIAutoSchedulerForwardAttempt(ctx, c, account, "legacy-model", time.Now(), &OpenAIForwardResult{
		UpstreamModel: " Final-Upstream ", UpstreamEndpoint: "/v1/chat/completions",
	}, nil)
	require.True(t, attempt.armed)
	require.NoError(t, recorder.Stop(context.Background()))

	records := legacy.snapshot()
	require.Len(t, records, 1)
	require.Equal(t, "final-upstream", records[0].ModelFamily)
	require.Equal(t, openAISchedulerHealthEndpointChat, records[0].Endpoint)
	require.Equal(t, OpenAIUpstreamTransportHTTPSSE, records[0].Transport)
}

func TestOpenAISchedulerHealthDirectOutcomeCarriesActualWSMetadata(t *testing.T) {
	legacy := &strictCollectingLegacyOutcomeSink{}
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(legacy, 2, 1)
	svc := &OpenAIGatewayService{openAIAutoSchedulerOutcomeRecorder: recorder}
	groupID := int64(11)
	ttft := 120
	svc.recordOpenAIAutoSchedulerOutcome(context.Background(), &Account{ID: 9, Platform: PlatformOpenAI}, &groupID, "legacy-model",
		OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventSuccess, TtfbMS: &ttft},
		openAIAutoSchedulerAttemptMetadata{
			ModelFamily: " GPT-5.4 ", Endpoint: openAISchedulerHealthEndpointResponses,
			Transport: OpenAIUpstreamTransportResponsesWebsocketV2Ingress,
		})
	require.NoError(t, recorder.Stop(context.Background()))

	records := legacy.snapshot()
	require.Len(t, records, 1)
	require.Equal(t, "gpt-5.4", records[0].ModelFamily)
	require.Equal(t, openAISchedulerHealthEndpointResponses, records[0].Endpoint)
	require.Equal(t, OpenAIUpstreamTransportResponsesWebsocketV2, records[0].Transport)
}

func TestOpenAISchedulerHealthCompositePreservesProductionSlowClassification(t *testing.T) {
	settings := enabledOpenAIAutoSchedulerSettings()
	settings.SlowThresholdMS = 100
	settings.SevereSlowThresholdMS = 200
	legacyRepo := &fakeOpenAIAutoSchedulerRepo{groups: map[int64]Group{10: {
		ID: 10, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true,
	}}}
	settingsProvider := fakeOpenAIAutoSchedulerSettingsProvider{settings: settings}
	legacy := NewOpenAIAutoSchedulerService(legacyRepo, settingsProvider)
	healthRepo := &recordingOpenAISchedulerHealthRepository{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}}
	healthSink := NewOpenAISchedulerHealthEventSink(healthRepo, settingsProvider)
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(legacy, 2, 1, healthSink)
	ttft := 150
	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{
		AccountID: 1, GroupID: 10, Model: "gpt-5.4", ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: OpenAIUpstreamTransportHTTPSSE,
		EventType: OpenAIAutoSchedulerEventSuccess, TtfbMS: &ttft,
	}))
	require.NoError(t, recorder.Stop(context.Background()))

	require.Len(t, legacyRepo.events, 1)
	require.Equal(t, OpenAIAutoSchedulerEventSlow, legacyRepo.events[0].EventType)
	for _, state := range healthRepo.snapshot() {
		require.Equal(t, 1, state.ConsecutiveSlow)
		require.Equal(t, OpenAIAutoSchedulerStateObserving, state.State)
	}
}

func TestOpenAISchedulerHealthWSIngressFailureUsesMappedAttemptModel(t *testing.T) {
	account := &Account{
		ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
			"model_mapping": map[string]any{
				"client-alias": "mapped-upstream-model",
			},
		},
	}
	groupID := int64(30)
	for _, tt := range []struct {
		name      string
		transport OpenAIUpstreamTransport
	}{
		{name: "HTTP bridge failure", transport: OpenAIUpstreamTransportHTTPSSE},
		{name: "WS relay failure", transport: OpenAIUpstreamTransportResponsesWebsocketV2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			legacy := &strictCollectingLegacyOutcomeSink{}
			recorder := NewOpenAIAutoSchedulerOutcomeRecorder(legacy, 1, 1)
			svc := &OpenAIGatewayService{openAIAutoSchedulerOutcomeRecorder: recorder}
			attemptMetadata := openAIWSIngressAttemptMetadata(account, "client-alias", tt.transport)
			svc.recordOpenAIAutoSchedulerOutcome(context.Background(), account, &groupID, "client-alias",
				OpenAIAutoSchedulerRecordInput{EventType: OpenAIAutoSchedulerEventError}, attemptMetadata)
			require.NoError(t, recorder.Stop(context.Background()))

			records := legacy.snapshot()
			require.Len(t, records, 1)
			require.Equal(t, "client-alias", records[0].Model, "legacy model remains client-facing")
			require.Equal(t, "mapped-upstream-model", records[0].ModelFamily)
			require.Equal(t, openAISchedulerHealthEndpointResponses, records[0].Endpoint)
			require.Equal(t, tt.transport, records[0].Transport)
		})
	}
}

func TestOpenAIWSIngressAttemptMetadataMapsBeforeOAuthAliasNormalization(t *testing.T) {
	account := &Account{
		ID: 21, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"custom-alias": "gpt-5.1"},
		},
	}

	metadata := openAIWSIngressAttemptMetadata(account, "custom-alias", OpenAIUpstreamTransportResponsesWebsocketV2)
	require.Equal(t, "gpt-5.4", metadata.ModelFamily)
}
