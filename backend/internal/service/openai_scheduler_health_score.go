package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	HealthSourceReal  = "real"
	HealthSourceProbe = "probe"

	openAISchedulerHealthRealAlpha          = 0.2
	openAISchedulerHealthProbeAlpha         = 0.1
	openAISchedulerHealthStateTTL           = 30 * time.Minute
	openAISchedulerHealthRealFreshSeconds   = 300
	openAISchedulerHealthEndpointResponses  = "responses"
	openAISchedulerHealthEndpointChat       = "chat_completions"
	openAISchedulerHealthEndpointEmbeddings = "embeddings"
	openAISchedulerHealthEndpointImagesGen  = "images_generations"
	openAISchedulerHealthEndpointImagesEdit = "images_edits"
)

type OpenAISchedulerHealthEvent struct {
	Source     string
	EventType  string
	TTFTMS     float64
	StatusCode *int
	OccurredAt time.Time
}

type OpenAISchedulerHealthSettings struct {
	RealSampleAlpha                  float64
	ProbeSampleAlpha                 float64
	StateTTL                         time.Duration
	RealSampleFreshSeconds           int
	SlowThresholdMS                  int
	SevereSlowThresholdMS            int
	ConsecutiveSlowBreakerThreshold  int
	ConsecutiveErrorBreakerThreshold int
	CooldownSeconds                  int
	HalfOpenSuccessThreshold         int
}

type openAIAutoSchedulerAttemptMetadata struct {
	ModelFamily string
	Endpoint    string
	Transport   OpenAIUpstreamTransport
}

type OpenAISchedulerHealthEventSink struct {
	repo             OpenAISchedulerHealthRepository
	settingsProvider OpenAIAutoSchedulerSettingsProvider
	now              func() time.Time
	mu               sync.Mutex
	keyLocks         map[OpenAISchedulerHealthKey]*sync.Mutex
	metadataMissing  atomic.Uint64
}

func NewOpenAISchedulerHealthEventSink(
	repo OpenAISchedulerHealthRepository,
	settingsProvider OpenAIAutoSchedulerSettingsProvider,
) *OpenAISchedulerHealthEventSink {
	return &OpenAISchedulerHealthEventSink{
		repo: repo, settingsProvider: settingsProvider, now: time.Now,
		keyLocks: map[OpenAISchedulerHealthKey]*sync.Mutex{},
	}
}

func (s *OpenAISchedulerHealthEventSink) Record(ctx context.Context, input OpenAIAutoSchedulerRecordInput) error {
	if s == nil || s.repo == nil {
		return nil
	}
	key := normalizeOpenAISchedulerHealthKey(OpenAISchedulerHealthKey{
		AccountID: input.AccountID, ModelFamily: input.ModelFamily, Endpoint: input.Endpoint, Transport: string(input.Transport),
	})
	if !isCompleteOpenAISchedulerHealthKey(key) {
		missing := s.metadataMissing.Add(1)
		if shouldLogOpenAIAutoSchedulerOutcomeRecorderCount(missing) {
			slog.Warn("OpenAI scheduler health outcome metadata missing", "count", missing, "account_id", input.AccountID)
		}
		return nil
	}

	release := s.lockKey(key)
	defer release()
	states, err := s.repo.GetBatch(ctx, []OpenAISchedulerHealthKey{key})
	if err != nil {
		return err
	}
	now := s.now()
	current := states[key]
	current.Key = key
	event := OpenAISchedulerHealthEvent{
		Source: HealthSourceReal, EventType: input.EventType, TTFTMS: openAISchedulerOutcomeTTFTMS(input),
		StatusCode: input.StatusCode, OccurredAt: now,
	}
	settings := DefaultOpenAISchedulerHealthSettings()
	if s.settingsProvider != nil {
		settings = openAISchedulerHealthSettingsFromAutoScheduler(s.settingsProvider.GetOpenAIAutoSchedulerSettings(ctx))
	}
	return s.repo.Upsert(ctx, ApplyOpenAISchedulerHealthEvent(now, current, event, settings))
}

func (s *OpenAISchedulerHealthEventSink) lockKey(key OpenAISchedulerHealthKey) func() {
	s.mu.Lock()
	lock := s.keyLocks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		s.keyLocks[key] = lock
	}
	s.mu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func DefaultOpenAISchedulerHealthSettings() OpenAISchedulerHealthSettings {
	return openAISchedulerHealthSettingsFromAutoScheduler(DefaultOpenAIAutoSchedulerSettings())
}

func openAISchedulerHealthSettingsFromAutoScheduler(settings OpenAIAutoSchedulerSettings) OpenAISchedulerHealthSettings {
	settings = normalizeOpenAIAutoSchedulerSettings(settings)
	return OpenAISchedulerHealthSettings{
		RealSampleAlpha:                  openAISchedulerHealthRealAlpha,
		ProbeSampleAlpha:                 openAISchedulerHealthProbeAlpha,
		StateTTL:                         openAISchedulerHealthStateTTL,
		RealSampleFreshSeconds:           openAISchedulerHealthRealFreshSeconds,
		SlowThresholdMS:                  settings.SlowThresholdMS,
		SevereSlowThresholdMS:            settings.SevereSlowThresholdMS,
		ConsecutiveSlowBreakerThreshold:  settings.ConsecutiveSlowBreakerThreshold,
		ConsecutiveErrorBreakerThreshold: settings.ConsecutiveErrorBreakerThreshold,
		CooldownSeconds:                  settings.CooldownSeconds,
		HalfOpenSuccessThreshold:         settings.HalfOpenSuccessThreshold,
	}
}

func ApplyOpenAISchedulerHealthEvent(
	now time.Time,
	current OpenAISchedulerHealthSnapshot,
	event OpenAISchedulerHealthEvent,
	settings OpenAISchedulerHealthSettings,
) OpenAISchedulerHealthSnapshot {
	settings = normalizeOpenAISchedulerHealthSettings(settings)
	if !current.ExpiresAt.IsZero() && !now.Before(current.ExpiresAt) {
		current = OpenAISchedulerHealthSnapshot{Key: current.Key}
	}
	current.State = normalizeOpenAIAutoSchedulerState(current.State)
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}

	previousSamples := current.RealSampleCount + current.ProbeSampleCount
	alpha := settings.RealSampleAlpha
	if event.Source == HealthSourceProbe {
		alpha = settings.ProbeSampleAlpha
		current.ProbeSampleCount++
		current.LastProbeAt = latestOpenAISchedulerHealthTime(current.LastProbeAt, event.OccurredAt)
	} else {
		current.RealSampleCount++
		current.LastRealAt = latestOpenAISchedulerHealthTime(current.LastRealAt, event.OccurredAt)
	}

	probeBlockedByReal := event.Source == HealthSourceProbe && current.LastRealAt != nil &&
		current.LastRealAt.After(now.Add(-time.Duration(settings.RealSampleFreshSeconds)*time.Second))
	if event.TTFTMS > 0 && !probeBlockedByReal {
		current.PredictedTTFTMS = openAISchedulerHealthEWMA(current.PredictedTTFTMS, event.TTFTMS, alpha, current.PredictedTTFTMS > 0)
	}

	isError := event.EventType == OpenAIAutoSchedulerEventError ||
		event.EventType == OpenAIAutoSchedulerEventProbeError ||
		event.EventType == OpenAIAutoSchedulerEventRateLimited
	isRateLimited := event.EventType == OpenAIAutoSchedulerEventRateLimited ||
		(event.StatusCode != nil && *event.StatusCode == 429)
	isServerError := event.StatusCode != nil && *event.StatusCode >= 500 && *event.StatusCode < 600
	current.ErrorRate = openAISchedulerHealthEWMA(current.ErrorRate, boolOpenAISchedulerHealthValue(isError), alpha, previousSamples > 0)
	current.RateLimitedRate = openAISchedulerHealthEWMA(current.RateLimitedRate, boolOpenAISchedulerHealthValue(isRateLimited), alpha, previousSamples > 0)
	current.ServerErrorRate = openAISchedulerHealthEWMA(current.ServerErrorRate, boolOpenAISchedulerHealthValue(isServerError), alpha, previousSamples > 0)

	current = applyOpenAISchedulerHealthState(now, current, event.EventType, settings)
	current.ExpiresAt = now.Add(settings.StateTTL)
	return current
}

func applyOpenAISchedulerHealthState(
	now time.Time,
	current OpenAISchedulerHealthSnapshot,
	eventType string,
	settings OpenAISchedulerHealthSettings,
) OpenAISchedulerHealthSnapshot {
	isSuccess := eventType == OpenAIAutoSchedulerEventSuccess || eventType == OpenAIAutoSchedulerEventProbeSuccess
	if current.State == OpenAIAutoSchedulerStateOpen && openAIAutoSchedulerCooldownExpired(now, current.CooldownUntil) && isSuccess {
		current.State = OpenAIAutoSchedulerStateHalfOpen
		current.ConsecutiveSuccess = 1
		current.ConsecutiveSlow = 0
		current.ConsecutiveError = 0
		current.CooldownUntil = nil
		return current
	}

	switch eventType {
	case OpenAIAutoSchedulerEventSuccess, OpenAIAutoSchedulerEventProbeSuccess:
		current.ConsecutiveSlow = 0
		current.ConsecutiveError = 0
		current.ConsecutiveSuccess++
		if current.State == OpenAIAutoSchedulerStateHalfOpen && current.ConsecutiveSuccess >= settings.HalfOpenSuccessThreshold {
			current.State = OpenAIAutoSchedulerStateRunning
			current.CooldownUntil = nil
		} else if current.State != OpenAIAutoSchedulerStateOpen && current.State != OpenAIAutoSchedulerStateHalfOpen {
			current.State = OpenAIAutoSchedulerStateRunning
		}
	case OpenAIAutoSchedulerEventSlow, OpenAIAutoSchedulerEventSevereSlow:
		current.ConsecutiveSlow++
		current.ConsecutiveError = 0
		current.ConsecutiveSuccess = 0
		if current.ConsecutiveSlow >= settings.ConsecutiveSlowBreakerThreshold {
			current = openOpenAISchedulerHealthCircuit(now, current, settings)
		} else {
			current.State = OpenAIAutoSchedulerStateObserving
		}
	case OpenAIAutoSchedulerEventError, OpenAIAutoSchedulerEventProbeError, OpenAIAutoSchedulerEventRateLimited:
		current.ConsecutiveError++
		current.ConsecutiveSlow = 0
		current.ConsecutiveSuccess = 0
		if current.ConsecutiveError >= settings.ConsecutiveErrorBreakerThreshold || current.State == OpenAIAutoSchedulerStateHalfOpen {
			current = openOpenAISchedulerHealthCircuit(now, current, settings)
		} else {
			current.State = OpenAIAutoSchedulerStateObserving
		}
	}
	return current
}

func openOpenAISchedulerHealthCircuit(now time.Time, current OpenAISchedulerHealthSnapshot, settings OpenAISchedulerHealthSettings) OpenAISchedulerHealthSnapshot {
	cooldownUntil := now.Add(time.Duration(settings.CooldownSeconds) * time.Second)
	current.State = OpenAIAutoSchedulerStateOpen
	current.CooldownUntil = &cooldownUntil
	return current
}

func normalizeOpenAISchedulerHealthSettings(settings OpenAISchedulerHealthSettings) OpenAISchedulerHealthSettings {
	defaults := DefaultOpenAISchedulerHealthSettings()
	if settings.RealSampleAlpha <= 0 || settings.RealSampleAlpha > 1 {
		settings.RealSampleAlpha = defaults.RealSampleAlpha
	}
	if settings.ProbeSampleAlpha <= 0 || settings.ProbeSampleAlpha > 1 {
		settings.ProbeSampleAlpha = defaults.ProbeSampleAlpha
	}
	if settings.StateTTL <= 0 {
		settings.StateTTL = defaults.StateTTL
	}
	if settings.RealSampleFreshSeconds <= 0 {
		settings.RealSampleFreshSeconds = defaults.RealSampleFreshSeconds
	}
	legacy := normalizeOpenAIAutoSchedulerSettings(OpenAIAutoSchedulerSettings{
		SlowThresholdMS:                  settings.SlowThresholdMS,
		SevereSlowThresholdMS:            settings.SevereSlowThresholdMS,
		ConsecutiveSlowBreakerThreshold:  settings.ConsecutiveSlowBreakerThreshold,
		ConsecutiveErrorBreakerThreshold: settings.ConsecutiveErrorBreakerThreshold,
		CooldownSeconds:                  settings.CooldownSeconds,
		HalfOpenSuccessThreshold:         settings.HalfOpenSuccessThreshold,
	})
	settings.SlowThresholdMS = legacy.SlowThresholdMS
	settings.SevereSlowThresholdMS = legacy.SevereSlowThresholdMS
	settings.ConsecutiveSlowBreakerThreshold = legacy.ConsecutiveSlowBreakerThreshold
	settings.ConsecutiveErrorBreakerThreshold = legacy.ConsecutiveErrorBreakerThreshold
	settings.CooldownSeconds = legacy.CooldownSeconds
	settings.HalfOpenSuccessThreshold = legacy.HalfOpenSuccessThreshold
	return settings
}

func openAISchedulerHealthEWMA(previous, sample, alpha float64, hasPrevious bool) float64 {
	if !hasPrevious {
		return sample
	}
	return previous*(1-alpha) + sample*alpha
}

func boolOpenAISchedulerHealthValue(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func latestOpenAISchedulerHealthTime(current *time.Time, candidate time.Time) *time.Time {
	if current != nil && current.After(candidate) {
		copied := *current
		return &copied
	}
	return &candidate
}

func openAIAutoSchedulerHealthMetadataForAttempt(
	attempt openAIAutoSchedulerAttemptMetadata,
	result *OpenAIForwardResult,
) openAIAutoSchedulerAttemptMetadata {
	if result != nil {
		if model := strings.TrimSpace(result.UpstreamModel); model != "" {
			attempt.ModelFamily = model
		}
		if endpoint := strings.TrimSpace(result.UpstreamEndpoint); endpoint != "" {
			attempt.Endpoint = endpoint
		}
	}
	attempt.ModelFamily = strings.ToLower(strings.TrimSpace(attempt.ModelFamily))
	attempt.Endpoint = normalizeOpenAISchedulerHealthEndpoint(attempt.Endpoint)
	if attempt.Transport == OpenAIUpstreamTransportResponsesWebsocketV2Ingress {
		attempt.Transport = OpenAIUpstreamTransportResponsesWebsocketV2
	}
	return attempt
}

func normalizeOpenAISchedulerHealthEndpoint(endpoint string) string {
	switch strings.ToLower(strings.TrimSpace(endpoint)) {
	case openAISchedulerHealthEndpointResponses, "/v1/responses":
		return openAISchedulerHealthEndpointResponses
	case openAISchedulerHealthEndpointChat, "/v1/chat/completions":
		return openAISchedulerHealthEndpointChat
	case openAISchedulerHealthEndpointEmbeddings, "/v1/embeddings":
		return openAISchedulerHealthEndpointEmbeddings
	case openAISchedulerHealthEndpointImagesGen, "/v1/images/generations":
		return openAISchedulerHealthEndpointImagesGen
	case openAISchedulerHealthEndpointImagesEdit, "/v1/images/edits":
		return openAISchedulerHealthEndpointImagesEdit
	default:
		return ""
	}
}

func normalizeOpenAISchedulerHealthKey(key OpenAISchedulerHealthKey) OpenAISchedulerHealthKey {
	key.ModelFamily = strings.ToLower(strings.TrimSpace(key.ModelFamily))
	key.Endpoint = normalizeOpenAISchedulerHealthEndpoint(key.Endpoint)
	transport := OpenAIUpstreamTransport(strings.ToLower(strings.TrimSpace(key.Transport)))
	if transport == OpenAIUpstreamTransportResponsesWebsocketV2Ingress {
		transport = OpenAIUpstreamTransportResponsesWebsocketV2
	}
	switch transport {
	case OpenAIUpstreamTransportHTTPSSE,
		OpenAIUpstreamTransportResponsesWebsocket,
		OpenAIUpstreamTransportResponsesWebsocketV2:
		key.Transport = string(transport)
	default:
		key.Transport = ""
	}
	return key
}

func isCompleteOpenAISchedulerHealthKey(key OpenAISchedulerHealthKey) bool {
	return key.AccountID > 0 && key.ModelFamily != "" && key.Endpoint != "" && key.Transport != ""
}

func openAISchedulerOutcomeTTFTMS(input OpenAIAutoSchedulerRecordInput) float64 {
	if input.TtfbMS != nil && *input.TtfbMS > 0 {
		return float64(*input.TtfbMS)
	}
	if input.LatencyMS != nil && *input.LatencyMS > 0 {
		return float64(*input.LatencyMS)
	}
	return 0
}
