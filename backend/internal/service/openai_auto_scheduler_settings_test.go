package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type openAIAutoSchedulerSettingsRepoStub struct {
	values        map[string]string
	getValueCalls int
	getValueErr   error
	getValueFunc  func(context.Context) (string, error)
	mu            sync.Mutex
}

func (s *openAIAutoSchedulerSettingsRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *openAIAutoSchedulerSettingsRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	s.mu.Lock()
	s.getValueCalls++
	read := s.getValueFunc
	err := s.getValueErr
	value, ok := s.values[key]
	s.mu.Unlock()
	if read != nil {
		return read(ctx)
	}
	if err != nil {
		return "", err
	}
	if ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *openAIAutoSchedulerSettingsRepoStub) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getValueCalls
}

func (s *openAIAutoSchedulerSettingsRepoStub) setReadError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getValueErr = err
	s.getValueFunc = nil
}

func (s *openAIAutoSchedulerSettingsRepoStub) setReadFunc(read func(context.Context) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getValueFunc = read
	s.getValueErr = nil
}

func (s *openAIAutoSchedulerSettingsRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *openAIAutoSchedulerSettingsRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *openAIAutoSchedulerSettingsRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *openAIAutoSchedulerSettingsRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *openAIAutoSchedulerSettingsRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingService_OpenAIAutoSchedulerSettingsDefaultsAndNormalization(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{
		SettingKeyOpenAIAutoSchedulerSettings: `{"enabled":true,"probe_interval_seconds":0,"slow_threshold_ms":-1,"severe_slow_threshold_ms":1,"consecutive_slow_breaker_threshold":0,"consecutive_error_breaker_threshold":0,"cooldown_seconds":0,"half_open_success_threshold":0,"cost_weight":2,"recovery_step":0}`,
	}}
	svc := NewSettingService(repo, &config.Config{})

	settings := svc.GetOpenAIAutoSchedulerSettings(context.Background())

	require.True(t, settings.Enabled)
	require.Equal(t, OpenAIAutoSchedulerDefaultProbeModel, settings.ProbeModel)
	require.Equal(t, 60, settings.ProbeIntervalSeconds)
	require.Equal(t, 10000, settings.SlowThresholdMS)
	require.Equal(t, 10000, settings.SevereSlowThresholdMS)
	require.Equal(t, 3, settings.ConsecutiveSlowBreakerThreshold)
	require.Equal(t, 2, settings.ConsecutiveErrorBreakerThreshold)
	require.Equal(t, 120, settings.CooldownSeconds)
	require.Equal(t, 3, settings.HalfOpenSuccessThreshold)
	require.Equal(t, 1.0, settings.CostWeight)
	require.Equal(t, 800, settings.RecoveryStep)
}

func TestSettingService_ReportsAdvancedSchedulerEngineGate(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{openAIAdvancedSchedulerSettingKey: " true "}}
	svc := NewSettingService(repo, &config.Config{})

	require.True(t, svc.IsOpenAIAdvancedSchedulerEnabled(context.Background()))
	repo.values[openAIAdvancedSchedulerSettingKey] = "false"
	require.False(t, svc.IsOpenAIAdvancedSchedulerEnabled(context.Background()))
}

func TestNormalizeOpenAIAutoSchedulerSettings_BalancedDefaults(t *testing.T) {
	got := normalizeOpenAIAutoSchedulerSettings(OpenAIAutoSchedulerSettings{})

	require.Equal(t, "balanced", got.Mode)
	require.Equal(t, 3, got.TopK)
	require.InDelta(t, 0.03, got.ExplorationRate, 0.0001)
	require.Equal(t, 1000, got.SessionEscapeMinGapMS)
	require.InDelta(t, 0.25, got.SessionEscapeRatio, 0.0001)
	require.Equal(t, 1800, got.HealthTTLSeconds)
	require.Equal(t, 300, got.RealSampleFreshSeconds)
	require.Equal(t, 0, got.ProbeJitterSeconds)
}

func TestSettingService_OpenAIAutoSchedulerOldJSONDefaultsToShadow(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{
		SettingKeyOpenAIAutoSchedulerSettings: `{"enabled":true,"probe_interval_seconds":60}`,
	}}
	svc := NewSettingService(repo, &config.Config{})

	settings := svc.GetOpenAIAutoSchedulerSettings(context.Background())

	require.Equal(t, "balanced", settings.Mode)
	require.True(t, settings.ShadowMode)
	require.Equal(t, 3, settings.TopK)
	require.Equal(t, 1800, settings.HealthTTLSeconds)
	require.Equal(t, 300, settings.RealSampleFreshSeconds)
}

func TestSettingService_OpenAIAutoSchedulerExplicitShadowFalseSurvivesDecodeAndPersist(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{
		SettingKeyOpenAIAutoSchedulerSettings: `{"mode":"balanced","shadow_mode":false,"probe_interval_seconds":60}`,
	}}
	svc := NewSettingService(repo, &config.Config{})

	settings := svc.GetOpenAIAutoSchedulerSettings(context.Background())
	require.False(t, settings.ShadowMode)

	require.NoError(t, svc.SetOpenAIAutoSchedulerSettings(context.Background(), settings))
	var saved map[string]any
	require.NoError(t, json.Unmarshal([]byte(repo.values[SettingKeyOpenAIAutoSchedulerSettings]), &saved))
	require.Equal(t, false, saved["shadow_mode"])
}

func TestSettingService_OpenAIAutoSchedulerSettingsReadFailureUsesStaleLastGood(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{
		SettingKeyOpenAIAutoSchedulerSettings: `{"mode":"legacy","shadow_mode":false}`,
	}}
	svc := NewSettingService(repo, &config.Config{})

	first := svc.GetOpenAIAutoSchedulerSettings(context.Background())
	require.Equal(t, OpenAIAutoSchedulerModeLegacy, first.Mode)
	require.False(t, first.ShadowMode)
	expireOpenAIAutoSchedulerSettingsCache(t, svc)
	repo.setReadError(errors.New("database unavailable"))

	stale := svc.GetOpenAIAutoSchedulerSettings(context.Background())
	require.Equal(t, OpenAIAutoSchedulerModeLegacy, stale.Mode)
	require.False(t, stale.ShadowMode)
	require.Equal(t, 2, repo.calls())
}

func TestSettingService_OpenAIAutoSchedulerSettingsFailureWithoutLastGoodIsNotCached(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{}}
	repo.setReadError(errors.New("database unavailable"))
	svc := NewSettingService(repo, &config.Config{})

	first := svc.GetOpenAIAutoSchedulerSettings(context.Background())
	second := svc.GetOpenAIAutoSchedulerSettings(context.Background())

	require.Equal(t, OpenAIAutoSchedulerModeBalanced, first.Mode)
	require.True(t, first.ShadowMode)
	require.Equal(t, first, second)
	require.Equal(t, 2, repo.calls(), "failed defaults must not become a successful cache entry")
}

func TestSettingService_LoadOpenAIAutoSchedulerSettingsReturnsUnderlyingError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{}}
	repo.setReadError(wantErr)
	svc := NewSettingService(repo, &config.Config{})

	_, err := svc.loadOpenAIAutoSchedulerSettings(context.Background())

	require.ErrorIs(t, err, wantErr)
}

func TestSettingService_OpenAIAutoSchedulerSettingsTTLStartsAfterSuccessfulRead(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{}}
	repo.setReadFunc(func(context.Context) (string, error) {
		time.Sleep(200 * time.Millisecond)
		return `{"mode":"legacy","shadow_mode":false}`, nil
	})
	svc := NewSettingService(repo, &config.Config{})

	settings := svc.GetOpenAIAutoSchedulerSettings(context.Background())
	cached, _ := svc.openAIAutoSchedulerCache.Load().(*cachedOpenAIAutoSchedulerSettings)

	require.Equal(t, OpenAIAutoSchedulerModeLegacy, settings.Mode)
	require.NotNil(t, cached)
	require.Greater(t, time.Until(cached.expiresAt), openAIAutoSchedulerSettingsCacheTTL-100*time.Millisecond)
}

func TestSettingService_OpenAIAutoSchedulerSettingsSlowReadUsesLocalTimeout(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{}}
	repo.setReadFunc(func(ctx context.Context) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(3 * time.Second):
			return `{"mode":"legacy","shadow_mode":false}`, nil
		}
	})
	svc := NewSettingService(repo, &config.Config{})

	started := time.Now()
	settings := svc.GetOpenAIAutoSchedulerSettings(context.Background())
	elapsed := time.Since(started)

	require.Less(t, elapsed, 2800*time.Millisecond)
	require.Equal(t, OpenAIAutoSchedulerModeBalanced, settings.Mode)
	require.True(t, settings.ShadowMode)
	require.Equal(t, 1, repo.calls())
}

func TestSettingService_OpenAIAutoSchedulerSettingsConcurrentExpiryLoadsOnce(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{
		SettingKeyOpenAIAutoSchedulerSettings: `{"mode":"balanced","shadow_mode":false}`,
	}}
	svc := NewSettingService(repo, &config.Config{})
	require.False(t, svc.GetOpenAIAutoSchedulerSettings(context.Background()).ShadowMode)
	expireOpenAIAutoSchedulerSettingsCache(t, svc)
	repo.setReadFunc(func(context.Context) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return `{"mode":"legacy","shadow_mode":false}`, nil
	})

	const readers = 12
	var wg sync.WaitGroup
	wg.Add(readers)
	results := make(chan OpenAIAutoSchedulerSettings, readers)
	for range readers {
		go func() {
			defer wg.Done()
			results <- svc.GetOpenAIAutoSchedulerSettings(context.Background())
		}()
	}
	wg.Wait()
	close(results)
	for settings := range results {
		require.Equal(t, OpenAIAutoSchedulerModeLegacy, settings.Mode)
	}
	require.Equal(t, 2, repo.calls(), "initial load plus one shared refresh")
}

func TestSettingService_OpenAIAutoSchedulerInFlightRefreshCannotOverwriteSuccessfulUpdate(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	oldSettings := DefaultOpenAIAutoSchedulerSettings()
	oldSettings.Mode = OpenAIAutoSchedulerModeLegacy
	oldSettings.ShadowMode = true
	svc.openAIAutoSchedulerCache.Store(&cachedOpenAIAutoSchedulerSettings{
		settings: oldSettings, expiresAt: time.Now().Add(-time.Second),
	})
	readStarted := make(chan struct{})
	allowReadReturn := make(chan struct{})
	repo.setReadFunc(func(context.Context) (string, error) {
		close(readStarted)
		<-allowReadReturn
		return `{"mode":"legacy","shadow_mode":true}`, nil
	})
	getResult := make(chan OpenAIAutoSchedulerSettings, 1)
	go func() {
		getResult <- svc.GetOpenAIAutoSchedulerSettings(context.Background())
	}()
	<-readStarted

	updated := DefaultOpenAIAutoSchedulerSettings()
	updated.Mode = OpenAIAutoSchedulerModeBalanced
	updated.ShadowMode = false
	require.NoError(t, svc.SetOpenAIAutoSchedulerSettings(context.Background(), updated))
	close(allowReadReturn)

	refreshed := <-getResult
	require.Equal(t, OpenAIAutoSchedulerModeBalanced, refreshed.Mode)
	require.False(t, refreshed.ShadowMode)
	final := svc.GetOpenAIAutoSchedulerSettings(context.Background())
	require.Equal(t, OpenAIAutoSchedulerModeBalanced, final.Mode)
	require.False(t, final.ShadowMode)
}

func expireOpenAIAutoSchedulerSettingsCache(t *testing.T, svc *SettingService) {
	t.Helper()
	cached, _ := svc.openAIAutoSchedulerCache.Load().(*cachedOpenAIAutoSchedulerSettings)
	require.NotNil(t, cached)
	copy := *cached
	copy.expiresAt = time.Now().Add(-time.Second)
	svc.openAIAutoSchedulerCache.Store(&copy)
}

func TestSettingService_SetOpenAIAutoSchedulerSettingsPersistsNormalizedJSON(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	require.NoError(t, svc.SetOpenAIAutoSchedulerSettings(context.Background(), OpenAIAutoSchedulerSettings{
		Enabled:                          true,
		ProbeModel:                       "  gpt-5.5  ",
		ProbeIntervalSeconds:             -10,
		SlowThresholdMS:                  5000,
		SevereSlowThresholdMS:            4000,
		ConsecutiveSlowBreakerThreshold:  1,
		ConsecutiveErrorBreakerThreshold: 1,
		CooldownSeconds:                  30,
		HalfOpenSuccessThreshold:         1,
		CostWeight:                       -1,
		RecoveryStep:                     1200,
	}))

	var saved OpenAIAutoSchedulerSettings
	require.NoError(t, json.Unmarshal([]byte(repo.values[SettingKeyOpenAIAutoSchedulerSettings]), &saved))
	require.True(t, saved.Enabled)
	require.Equal(t, "gpt-5.5", saved.ProbeModel)
	require.Equal(t, 60, saved.ProbeIntervalSeconds)
	require.Equal(t, 5000, saved.SlowThresholdMS)
	require.Equal(t, 5000, saved.SevereSlowThresholdMS)
	require.Equal(t, 0.0, saved.CostWeight)
}
