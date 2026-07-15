package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type joinGroupSettingRepoStub struct {
	values map[string]string
}

func (s *joinGroupSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *joinGroupSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *joinGroupSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *joinGroupSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

func (s *joinGroupSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	if s.values == nil {
		s.values = make(map[string]string, len(settings))
	}
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *joinGroupSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	values := make(map[string]string, len(s.values))
	for key, value := range s.values {
		values[key] = value
	}
	return values, nil
}

func (s *joinGroupSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestSettingService_JoinGroupSettingsRoundTrip(t *testing.T) {
	repo := &joinGroupSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		JoinGroupEnabled:    true,
		JoinGroupURL:        " https://qm.qq.com/q/example ",
		JoinGroupPopupImage: " data:image/png;base64,QUJD ",
	})
	require.NoError(t, err)

	adminSettings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.True(t, adminSettings.JoinGroupEnabled)
	require.Equal(t, "https://qm.qq.com/q/example", adminSettings.JoinGroupURL)
	require.Equal(t, "data:image/png;base64,QUJD", adminSettings.JoinGroupPopupImage)

	publicSettings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, publicSettings.JoinGroupEnabled)
	require.Equal(t, "https://qm.qq.com/q/example", publicSettings.JoinGroupURL)
	require.Equal(t, "data:image/png;base64,QUJD", publicSettings.JoinGroupPopupImage)
}
