//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type anthropicAutoInspectSettingRepoStub struct {
	values map[string]string
}

func (s *anthropicAutoInspectSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *anthropicAutoInspectSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *anthropicAutoInspectSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *anthropicAutoInspectSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *anthropicAutoInspectSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *anthropicAutoInspectSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s *anthropicAutoInspectSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestSettingService_DefaultAnthropicAutoInspectSettings(t *testing.T) {
	t.Parallel()

	svc := NewSettingService(&anthropicAutoInspectSettingRepoStub{
		values: map[string]string{},
	}, &config.Config{})

	got, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.False(t, got.AnthropicAutoInspectEnabled)
	require.Equal(t, 1, got.AnthropicAutoInspectIntervalMinutes)
	require.Equal(t, 30, got.AnthropicAutoInspectErrorCooldownMinutes)
}
