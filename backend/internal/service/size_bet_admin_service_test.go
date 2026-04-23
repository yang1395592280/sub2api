package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type sizeBetSettingRepoStub struct {
	values  map[string]string
	updates map[string]string
	err     error
}

func (s *sizeBetSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *sizeBetSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *sizeBetSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *sizeBetSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[string]string, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	return out, nil
}

func (s *sizeBetSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	if s.err != nil {
		return s.err
	}
	s.updates = make(map[string]string, len(settings))
	for k, v := range settings {
		s.updates[k] = v
	}
	return nil
}

func (s *sizeBetSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *sizeBetSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestSizeBetAdminServiceGetSettingsReturnsDefaults(t *testing.T) {
	repo := &sizeBetSettingRepoStub{
		values: map[string]string{},
	}
	svc := NewSizeBetAdminService(repo)

	got, err := svc.GetSettings(context.Background())
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Equal(t, []int{2, 5, 10, 20}, got.AllowedStakes)
	require.InDelta(t, 45.0, got.ProbSmall, 0.001)
	require.InDelta(t, 10.0, got.ProbMid, 0.001)
	require.InDelta(t, 45.0, got.ProbBig, 0.001)
	require.InDelta(t, 2.0, got.OddsSmall, 0.001)
	require.InDelta(t, 10.0, got.OddsMid, 0.001)
	require.InDelta(t, 2.0, got.OddsBig, 0.001)
}

func TestSizeBetAdminServiceUpdateSettingsRejectsInvalidProbabilities(t *testing.T) {
	repo := &sizeBetSettingRepoStub{}
	svc := NewSizeBetAdminService(repo)

	err := svc.UpdateSettings(context.Background(), UpdateSizeBetSettingsRequest{
		Enabled:               true,
		RoundDurationSeconds:  60,
		BetCloseOffsetSeconds: 50,
		AllowedStakes:         []int{2, 5, 10, 20},
		Probabilities: SizeBetProbabilityConfig{
			Small: 44,
			Mid:   10,
			Big:   45,
		},
		Odds: SizeBetOddsConfig{
			Small: 2,
			Mid:   10,
			Big:   2,
		},
		RulesMarkdown: defaultSizeBetRulesMarkdown,
	})

	require.Error(t, err)
	require.Equal(t, "SIZE_BET_INVALID_PROBABILITIES", infraerrors.Reason(err))
	require.Nil(t, repo.updates)
}
