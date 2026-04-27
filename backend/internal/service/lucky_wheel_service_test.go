package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type luckyWheelSettingRepoStub struct {
	values map[string]string
	set    map[string]string
}

func (s *luckyWheelSettingRepoStub) Get(context.Context, string) (*Setting, error)    { return nil, nil }
func (s *luckyWheelSettingRepoStub) GetValue(context.Context, string) (string, error) { return "", nil }
func (s *luckyWheelSettingRepoStub) Set(context.Context, string, string) error        { return nil }
func (s *luckyWheelSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = s.values[key]
	}
	return result, nil
}
func (s *luckyWheelSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	s.set = settings
	return nil
}
func (s *luckyWheelSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}
func (s *luckyWheelSettingRepoStub) Delete(context.Context, string) error { return nil }

type luckyWheelRepoStub struct {
	assets      *GameCenterAssets
	lastInput   LuckyWheelApplySpinInput
	usedToday   int
	history     []LuckyWheelSpinRecord
	leaderboard []LuckyWheelLeaderboardItem
}

func (s *luckyWheelRepoStub) GetUserAssets(context.Context, int64) (*GameCenterAssets, error) {
	return s.assets, nil
}

func (s *luckyWheelRepoStub) CountUserSpinsOnDate(context.Context, int64, string) (int, error) {
	return s.usedToday, nil
}

func (s *luckyWheelRepoStub) ApplySpin(_ context.Context, input LuckyWheelApplySpinInput) (*LuckyWheelSpinRecord, error) {
	s.lastInput = input
	pointsBefore := s.assets.Points
	record := &LuckyWheelSpinRecord{
		ID:           1,
		UserID:       input.UserID,
		SpinDate:     input.SpinDate,
		SpinIndex:    s.usedToday + 1,
		PrizeKey:     input.Prize.Key,
		PrizeLabel:   input.Prize.Label,
		PrizeType:    input.Prize.Type,
		DeltaPoints:  input.Prize.DeltaPoints,
		PointsBefore: pointsBefore,
		PointsAfter:  pointsBefore + input.Prize.DeltaPoints,
		Probability:  input.Prize.Probability,
		CreatedAt:    input.TriggeredAt,
	}
	s.assets.Points = record.PointsAfter
	s.usedToday = record.SpinIndex
	s.history = append([]LuckyWheelSpinRecord{*record}, s.history...)
	return record, nil
}

func (s *luckyWheelRepoStub) ListUserSpins(context.Context, int64, pagination.PaginationParams) ([]LuckyWheelSpinRecord, *pagination.PaginationResult, error) {
	return s.history, &pagination.PaginationResult{Total: int64(len(s.history)), Page: 1, PageSize: 10, Pages: 1}, nil
}

func (s *luckyWheelRepoStub) ListAdminSpins(context.Context, pagination.PaginationParams, LuckyWheelAdminSpinFilter) ([]LuckyWheelSpinRecord, *pagination.PaginationResult, error) {
	return s.history, &pagination.PaginationResult{Total: int64(len(s.history)), Page: 1, PageSize: 10, Pages: 1}, nil
}

func (s *luckyWheelRepoStub) ListLeaderboard(context.Context, string, int) ([]LuckyWheelLeaderboardItem, error) {
	return s.leaderboard, nil
}

func TestLuckyWheelAdminServiceGetSettingsReturnsDefaults(t *testing.T) {
	svc := NewLuckyWheelAdminService(&luckyWheelSettingRepoStub{values: map[string]string{}})

	got, err := svc.GetSettings(context.Background())

	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Equal(t, 5, got.DailySpinLimit)
	require.Len(t, got.Prizes, 9)
	require.Equal(t, "thanks", got.Prizes[5].Key)
}

func TestLuckyWheelServiceSpinUsesConfiguredPrizeAndTracksRemaining(t *testing.T) {
	repo := &luckyWheelRepoStub{
		assets: &GameCenterAssets{Points: 500},
	}
	svc := NewLuckyWheelService(repo, &luckyWheelSettingRepoStub{values: map[string]string{}})
	svc.now = func() time.Time { return time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC) }
	svc.randomPercentage = func() float64 { return 0.1 }

	result, err := svc.Spin(context.Background(), 9)

	require.NoError(t, err)
	require.Equal(t, "jackpot_888", repo.lastInput.Prize.Key)
	require.Equal(t, 1, result.SpinsUsedToday)
	require.Equal(t, 4, result.SpinsRemainingToday)
	require.Equal(t, int64(1388), result.Record.PointsAfter)
	require.Equal(t, LuckyWheelPrizeReward, result.Record.PrizeType)
}

func TestLuckyWheelServiceSpinRejectsWhenPointsBelowMaxPenalty(t *testing.T) {
	repo := &luckyWheelRepoStub{
		assets: &GameCenterAssets{Points: 64},
	}
	svc := NewLuckyWheelService(repo, &luckyWheelSettingRepoStub{values: map[string]string{}})
	svc.now = func() time.Time { return time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC) }

	result, err := svc.Spin(context.Background(), 9)

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrLuckyWheelInsufficientPoints)
	require.Equal(t, int64(0), repo.lastInput.UserID)
}
