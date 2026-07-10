package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type rateLimit429SettingRepoStub struct {
	data map[string]string
}

func (s *rateLimit429SettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *rateLimit429SettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.data[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *rateLimit429SettingRepoStub) Set(_ context.Context, key, value string) error {
	if s.data == nil {
		s.data = map[string]string{}
	}
	s.data[key] = value
	return nil
}

func (s *rateLimit429SettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *rateLimit429SettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *rateLimit429SettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *rateLimit429SettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestOpenAIOverbrushSettings_DefaultsToThreshold10(t *testing.T) {
	repo := &rateLimit429SettingRepoStub{data: map[string]string{}}
	svc := NewSettingService(repo, nil)

	settings, err := svc.GetOpenAIOverbrushSettings(context.Background())

	require.NoError(t, err)
	require.Equal(t, 10, settings.Consecutive429Threshold)
}

func TestOpenAIOverbrushSettings_ClampsStoredValues(t *testing.T) {
	for name, tc := range map[string]struct {
		raw  int
		want int
	}{
		"below min": {raw: 0, want: 1},
		"above max": {raw: 101, want: 100},
		"valid":     {raw: 17, want: 17},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &rateLimit429SettingRepoStub{data: map[string]string{}}
			body, err := json.Marshal(OpenAIOverbrushSettings{Consecutive429Threshold: tc.raw})
			require.NoError(t, err)
			repo.data[SettingKeyOpenAIOverbrushSettings] = string(body)
			svc := NewSettingService(repo, nil)

			settings, err := svc.GetOpenAIOverbrushSettings(context.Background())

			require.NoError(t, err)
			require.Equal(t, tc.want, settings.Consecutive429Threshold)
		})
	}
}

func TestOpenAIOverbrushSettings_SetRejectsInvalidEnabledValues(t *testing.T) {
	repo := &rateLimit429SettingRepoStub{data: map[string]string{}}
	svc := NewSettingService(repo, nil)

	require.ErrorContains(t,
		svc.SetOpenAIOverbrushSettings(context.Background(), &OpenAIOverbrushSettings{Consecutive429Threshold: 0}),
		"consecutive_429_threshold must be between 1-100",
	)
	require.ErrorContains(t,
		svc.SetOpenAIOverbrushSettings(context.Background(), &OpenAIOverbrushSettings{Consecutive429Threshold: 101}),
		"consecutive_429_threshold must be between 1-100",
	)
}

func TestOpenAIOverbrushSettings_SetPersistsValidValue(t *testing.T) {
	repo := &rateLimit429SettingRepoStub{data: map[string]string{}}
	svc := NewSettingService(repo, nil)

	err := svc.SetOpenAIOverbrushSettings(context.Background(), &OpenAIOverbrushSettings{Consecutive429Threshold: 12})

	require.NoError(t, err)
	require.JSONEq(t, `{"consecutive_429_threshold":12}`, repo.data[SettingKeyOpenAIOverbrushSettings])
}
