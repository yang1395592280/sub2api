package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type openAIAutoSchedulerSettingsRepoStub struct {
	values map[string]string
}

func (s *openAIAutoSchedulerSettingsRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *openAIAutoSchedulerSettingsRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
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
