package service

import (
	"context"
	"math/rand"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type checkinSettingRepoStub struct {
	values map[string]string
}

func (s *checkinSettingRepoStub) Get(context.Context, string) (*Setting, error) { panic("unexpected Get call") }
func (s *checkinSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}
func (s *checkinSettingRepoStub) Set(context.Context, string, string) error { panic("unexpected Set call") }
func (s *checkinSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}
func (s *checkinSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}
func (s *checkinSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}
func (s *checkinSettingRepoStub) Delete(context.Context, string) error { panic("unexpected Delete call") }

type checkinRepoStub struct {
	hasChecked           bool
	records              []CheckinRecord
	getByDateRecord      *CheckinRecord
	totalCount           int64
	totalRewardPoints    int64
	createResult         *CheckinRecord
	createErr            error
	applyBonusResult     *CheckinRecord
	applyBonusErr        error
	lastCreateRecord     *CheckinRecord
	lastApplyBonusDate   string
	lastApplyBonusOutome string
	lastApplyBonusDelta  int64
}

func (s *checkinRepoStub) HasCheckedInOnDate(context.Context, int64, string) (bool, error) {
	return s.hasChecked, nil
}
func (s *checkinRepoStub) CreateAndCredit(_ context.Context, record *CheckinRecord) (*CheckinRecord, error) {
	s.lastCreateRecord = record
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createResult != nil {
		return s.createResult, nil
	}
	return record, nil
}
func (s *checkinRepoStub) ListByUserAndDateRange(context.Context, int64, string, string) ([]CheckinRecord, error) {
	return s.records, nil
}
func (s *checkinRepoStub) GetByUserAndDate(context.Context, int64, string) (*CheckinRecord, error) {
	return s.getByDateRecord, nil
}
func (s *checkinRepoStub) ApplyBonusOutcome(_ context.Context, _ int64, date, outcome string, delta int64) (*CheckinRecord, error) {
	s.lastApplyBonusDate = date
	s.lastApplyBonusOutome = outcome
	s.lastApplyBonusDelta = delta
	if s.applyBonusErr != nil {
		return nil, s.applyBonusErr
	}
	return s.applyBonusResult, nil
}
func (s *checkinRepoStub) GetUserTotals(context.Context, int64) (int64, int64, error) {
	return s.totalCount, s.totalRewardPoints, nil
}
func (s *checkinRepoStub) ListAdminRecords(context.Context, int, int, string, string, string, string, string) ([]AdminCheckinRecord, int64, error) {
	panic("unexpected ListAdminRecords call")
}
func (s *checkinRepoStub) GetAdminOverview(context.Context, AdminCheckinAnalyticsFilter) (AdminCheckinOverview, error) {
	panic("unexpected GetAdminOverview call")
}
func (s *checkinRepoStub) GetAdminTrend(context.Context, AdminCheckinAnalyticsFilter) ([]AdminCheckinTrendPoint, error) {
	panic("unexpected GetAdminTrend call")
}
func (s *checkinRepoStub) GetAdminRewardDistribution(context.Context, AdminCheckinAnalyticsFilter) ([]AdminCheckinRewardBucket, error) {
	panic("unexpected GetAdminRewardDistribution call")
}
func (s *checkinRepoStub) GetAdminTopUsers(context.Context, AdminCheckinAnalyticsFilter) ([]AdminCheckinTopUser, error) {
	panic("unexpected GetAdminTopUsers call")
}

func newCheckinSettings(enabled bool, minReward, maxReward string) *checkinSettingRepoStub {
	return &checkinSettingRepoStub{
		values: map[string]string{
			SettingKeyCheckinEnabled:               map[bool]string{true: "true", false: "false"}[enabled],
			SettingKeyCheckinMinReward:             minReward,
			SettingKeyCheckinMaxReward:             maxReward,
			SettingKeyCheckinDistributionEnabled:   "false",
			SettingKeyCheckinDistributionConfig:    "[]",
			SettingKeyCheckinLuckyBonusEnabled:     "false",
			SettingKeyCheckinLuckyBonusSuccessRate: "50",
		},
	}
}

func newCheckinSettingsWithDistribution(enabled bool, minReward, maxReward, distribution string) *checkinSettingRepoStub {
	settings := newCheckinSettings(enabled, minReward, maxReward)
	settings.values[SettingKeyCheckinDistributionEnabled] = "true"
	settings.values[SettingKeyCheckinDistributionConfig] = distribution
	return settings
}

func newCheckinSettingsWithBonus(enabled bool, minReward, maxReward string, bonusEnabled bool, successRate string) *checkinSettingRepoStub {
	settings := newCheckinSettings(enabled, minReward, maxReward)
	settings.values[SettingKeyCheckinLuckyBonusEnabled] = map[bool]string{true: "true", false: "false"}[bonusEnabled]
	settings.values[SettingKeyCheckinLuckyBonusSuccessRate] = successRate
	return settings
}

func TestCheckinServiceCheckinReturnsDisabledErrorWhenFeatureOff(t *testing.T) {
	t.Parallel()

	svc := NewCheckinService(&checkinRepoStub{}, newCheckinSettings(false, "2", "20"), nil, nil)
	_, err := svc.Checkin(context.Background(), 42, "Asia/Shanghai")
	require.ErrorIs(t, err, ErrCheckinDisabled)
}

func TestCheckinServiceCheckinCreatesRewardWithinConfiguredRange(t *testing.T) {
	t.Parallel()

	repo := &checkinRepoStub{}
	svc := NewCheckinService(repo, newCheckinSettings(true, "2", "20"), nil, nil)
	record, err := svc.Checkin(context.Background(), 42, "Asia/Shanghai")

	require.NoError(t, err)
	require.NotNil(t, record)
	require.NotNil(t, repo.lastCreateRecord)
	require.Equal(t, int64(42), repo.lastCreateRecord.UserID)
	require.Equal(t, timezone.NowInUserLocation("Asia/Shanghai").Format("2006-01-02"), repo.lastCreateRecord.CheckinDate)
	require.GreaterOrEqual(t, repo.lastCreateRecord.RewardPoints, int64(2))
	require.LessOrEqual(t, repo.lastCreateRecord.RewardPoints, int64(20))
}

func TestCheckinServiceUsesDistributionConfigWhenEnabled(t *testing.T) {
	t.Parallel()

	repo := &checkinRepoStub{}
	svc := NewCheckinService(
		repo,
		newCheckinSettingsWithDistribution(true, "10", "100", `[{"start_percent":0,"end_percent":25,"weight":80},{"start_percent":25,"end_percent":50,"weight":10},{"start_percent":50,"end_percent":75,"weight":5},{"start_percent":75,"end_percent":90,"weight":4},{"start_percent":90,"end_percent":100,"weight":1}]`),
		nil,
		nil,
	)
	svc.randSource = rand.New(rand.NewSource(1))

	_, err := svc.Checkin(context.Background(), 42, "Asia/Shanghai")
	require.NoError(t, err)
	require.GreaterOrEqual(t, repo.lastCreateRecord.RewardPoints, int64(10))
	require.LessOrEqual(t, repo.lastCreateRecord.RewardPoints, int64(33))
}

func TestCheckinServiceGetStatusAggregatesMonthlyRecordsAndTotals(t *testing.T) {
	t.Parallel()

	repo := &checkinRepoStub{
		hasChecked: true,
		records: []CheckinRecord{
			{CheckinDate: "2026-04-02", RewardPoints: 10},
			{CheckinDate: "2026-04-01", RewardPoints: 20},
		},
		totalCount:        12,
		totalRewardPoints: 345,
	}
	svc := NewCheckinService(repo, newCheckinSettings(true, "2", "20"), nil, nil)

	status, err := svc.GetStatus(context.Background(), 42, "2026-04", "Asia/Shanghai")
	require.NoError(t, err)
	require.True(t, status.Enabled)
	require.Equal(t, int64(2), status.MinRewardPoints)
	require.Equal(t, int64(20), status.MaxRewardPoints)
	require.True(t, status.Stats.CheckedInToday)
	require.Equal(t, int64(12), status.Stats.TotalCheckins)
	require.Equal(t, int64(345), status.Stats.TotalRewardPoints)
	require.Len(t, status.Stats.Records, 2)
}

func TestCheckinServicePlayLuckyBonusWinsAndCreditsExtraReward(t *testing.T) {
	t.Parallel()

	today := timezone.NowInUserLocation("Asia/Shanghai").Format("2006-01-02")
	repo := &checkinRepoStub{
		getByDateRecord: &CheckinRecord{
			UserID:           42,
			CheckinDate:      today,
			RewardPoints:     10,
			BaseRewardPoints: 10,
			BonusStatus:      CheckinBonusStatusNone,
		},
		applyBonusResult: &CheckinRecord{
			UserID:           42,
			CheckinDate:      today,
			RewardPoints:     20,
			BaseRewardPoints: 10,
			BonusStatus:      CheckinBonusStatusWin,
			BonusDeltaPoints: 10,
		},
	}
	svc := NewCheckinService(repo, newCheckinSettingsWithBonus(true, "10", "10", true, "100"), nil, nil)

	record, err := svc.PlayLuckyBonus(context.Background(), 42, "Asia/Shanghai")
	require.NoError(t, err)
	require.Equal(t, CheckinBonusStatusWin, repo.lastApplyBonusOutome)
	require.Equal(t, int64(10), repo.lastApplyBonusDelta)
	require.Equal(t, int64(20), record.RewardPoints)
}

func TestCheckinServicePlayLuckyBonusLosesOnlyHalfBaseReward(t *testing.T) {
	t.Parallel()

	today := timezone.NowInUserLocation("Asia/Shanghai").Format("2006-01-02")
	repo := &checkinRepoStub{
		getByDateRecord: &CheckinRecord{
			UserID:           42,
			CheckinDate:      today,
			RewardPoints:     10,
			BaseRewardPoints: 10,
			BonusStatus:      CheckinBonusStatusNone,
		},
		applyBonusResult: &CheckinRecord{
			UserID:           42,
			CheckinDate:      today,
			RewardPoints:     5,
			BaseRewardPoints: 10,
			BonusStatus:      CheckinBonusStatusLose,
			BonusDeltaPoints: -5,
		},
	}
	svc := NewCheckinService(repo, newCheckinSettingsWithBonus(true, "10", "10", true, "0"), nil, nil)

	record, err := svc.PlayLuckyBonus(context.Background(), 42, "Asia/Shanghai")
	require.NoError(t, err)
	require.Equal(t, CheckinBonusStatusLose, repo.lastApplyBonusOutome)
	require.Equal(t, int64(-5), repo.lastApplyBonusDelta)
	require.Equal(t, int64(5), record.RewardPoints)
	require.Equal(t, int64(-5), record.BonusDeltaPoints)
}
