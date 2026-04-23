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
	if s.values == nil {
		s.values = make(map[string]string, len(settings))
	}
	s.updates = make(map[string]string, len(settings))
	for k, v := range settings {
		s.updates[k] = v
		s.values[k] = v
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
	require.NotEmpty(t, got.RulesMarkdown)
}

func TestSizeBetAdminServiceUpdateSettingsPersistsAndReadsBack(t *testing.T) {
	repo := &sizeBetSettingRepoStub{
		values: map[string]string{},
	}
	svc := NewSizeBetAdminService(repo)

	err := svc.UpdateSettings(context.Background(), UpdateSizeBetSettingsRequest{
		Enabled:               false,
		RoundDurationSeconds:  30,
		BetCloseOffsetSeconds: 20,
		AllowedStakes:         []int{3, 9, 27},
		Probabilities: SizeBetProbabilityConfig{
			Small: 40,
			Mid:   20,
			Big:   40,
		},
		Odds: SizeBetOddsConfig{
			Small: 1.8,
			Mid:   8.5,
			Big:   1.8,
		},
		RulesMarkdown: "完整规则内容",
	})
	require.NoError(t, err)

	got, err := svc.GetSettings(context.Background())
	require.NoError(t, err)
	require.False(t, got.Enabled)
	require.Equal(t, 30, got.RoundDurationSeconds)
	require.Equal(t, 20, got.BetCloseOffsetSeconds)
	require.Equal(t, []int{3, 9, 27}, got.AllowedStakes)
	require.InDelta(t, 40.0, got.ProbSmall, 0.001)
	require.InDelta(t, 20.0, got.ProbMid, 0.001)
	require.InDelta(t, 40.0, got.ProbBig, 0.001)
	require.InDelta(t, 1.8, got.OddsSmall, 0.001)
	require.InDelta(t, 8.5, got.OddsMid, 0.001)
	require.InDelta(t, 1.8, got.OddsBig, 0.001)
	require.Equal(t, "完整规则内容", got.RulesMarkdown)
}

func TestSizeBetAdminServiceGetSettingsClampsFallbackCloseOffsetForShortRound(t *testing.T) {
	testCases := []struct {
		name   string
		values map[string]string
	}{
		{
			name: "missing close offset falls back safely",
			values: map[string]string{
				SettingKeySizeBetRoundDurationSeconds: "5",
			},
		},
		{
			name: "invalid close offset falls back safely",
			values: map[string]string{
				SettingKeySizeBetRoundDurationSeconds:  "5",
				SettingKeySizeBetBetCloseOffsetSeconds: "50",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewSizeBetAdminService(&sizeBetSettingRepoStub{values: tc.values})

			got, err := svc.GetSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, 5, got.RoundDurationSeconds)
			require.Equal(t, 4, got.BetCloseOffsetSeconds)
		})
	}
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

func TestSizeBetAdminServiceUpdateSettingsRejectsBlankRulesMarkdown(t *testing.T) {
	repo := &sizeBetSettingRepoStub{}
	svc := NewSizeBetAdminService(repo)

	err := svc.UpdateSettings(context.Background(), UpdateSizeBetSettingsRequest{
		Enabled:               true,
		RoundDurationSeconds:  60,
		BetCloseOffsetSeconds: 50,
		AllowedStakes:         []int{2, 5, 10, 20},
		Probabilities: SizeBetProbabilityConfig{
			Small: 45,
			Mid:   10,
			Big:   45,
		},
		Odds: SizeBetOddsConfig{
			Small: 2,
			Mid:   10,
			Big:   2,
		},
		RulesMarkdown: " \n\t ",
	})

	require.Error(t, err)
	require.Equal(t, "SIZE_BET_RULES_MARKDOWN_REQUIRED", infraerrors.Reason(err))
	require.Nil(t, repo.updates)
}
