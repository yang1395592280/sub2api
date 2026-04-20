package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type checkinSettingRepoStub struct {
	values map[string]string
}

func (s *checkinSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *checkinSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *checkinSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *checkinSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *checkinSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *checkinSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *checkinSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type checkinRepoStub struct {
	hasChecked       bool
	hasCheckedErr    error
	records          []CheckinRecord
	recordsErr       error
	totalCount       int64
	totalReward      float64
	totalsErr        error
	createResult     *CheckinRecord
	createErr        error
	lastCreateRecord *CheckinRecord
	lastCreateUserID int64
}

func (s *checkinRepoStub) HasCheckedInOnDate(_ context.Context, userID int64, date string) (bool, error) {
	s.lastCreateUserID = userID
	if s.hasCheckedErr != nil {
		return false, s.hasCheckedErr
	}
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

func (s *checkinRepoStub) ListByUserAndDateRange(_ context.Context, userID int64, startDate, endDate string) ([]CheckinRecord, error) {
	s.lastCreateUserID = userID
	if s.recordsErr != nil {
		return nil, s.recordsErr
	}
	return s.records, nil
}

func (s *checkinRepoStub) GetUserTotals(_ context.Context, userID int64) (int64, float64, error) {
	s.lastCreateUserID = userID
	if s.totalsErr != nil {
		return 0, 0, s.totalsErr
	}
	return s.totalCount, s.totalReward, nil
}

func newCheckinSettings(enabled bool, minReward, maxReward string) *checkinSettingRepoStub {
	return &checkinSettingRepoStub{
		values: map[string]string{
			SettingKeyCheckinEnabled:   map[bool]string{true: "true", false: "false"}[enabled],
			SettingKeyCheckinMinReward: minReward,
			SettingKeyCheckinMaxReward: maxReward,
		},
	}
}

func TestCheckinServiceCheckinReturnsDisabledErrorWhenFeatureOff(t *testing.T) {
	t.Parallel()

	svc := NewCheckinService(&checkinRepoStub{}, newCheckinSettings(false, "0.002", "0.020"), nil, nil)

	_, err := svc.Checkin(context.Background(), 42, "Asia/Shanghai")

	require.ErrorIs(t, err, ErrCheckinDisabled)
}

func TestCheckinServiceCheckinReturnsAlreadyTodayWhenExistingRecordFound(t *testing.T) {
	t.Parallel()

	repo := &checkinRepoStub{hasChecked: true}
	svc := NewCheckinService(repo, newCheckinSettings(true, "0.002", "0.020"), nil, nil)

	_, err := svc.Checkin(context.Background(), 42, "Asia/Shanghai")

	require.ErrorIs(t, err, ErrCheckinAlreadyToday)
	require.Nil(t, repo.lastCreateRecord)
}

func TestCheckinServiceCheckinCreatesRewardWithinConfiguredRange(t *testing.T) {
	t.Parallel()

	repo := &checkinRepoStub{}
	svc := NewCheckinService(repo, newCheckinSettings(true, "0.002", "0.020"), nil, nil)

	record, err := svc.Checkin(context.Background(), 42, "Asia/Shanghai")

	require.NoError(t, err)
	require.NotNil(t, record)
	require.NotNil(t, repo.lastCreateRecord)
	require.Equal(t, int64(42), repo.lastCreateRecord.UserID)
	require.Equal(t, timezone.NowInUserLocation("Asia/Shanghai").Format("2006-01-02"), repo.lastCreateRecord.CheckinDate)
	require.Equal(t, "Asia/Shanghai", repo.lastCreateRecord.UserTimezone)
	require.GreaterOrEqual(t, repo.lastCreateRecord.RewardAmount, 0.002)
	require.LessOrEqual(t, repo.lastCreateRecord.RewardAmount, 0.020)
}

func TestCheckinServiceGetStatusAggregatesMonthlyRecordsAndTotals(t *testing.T) {
	t.Parallel()

	repo := &checkinRepoStub{
		hasChecked: true,
		records: []CheckinRecord{
			{CheckinDate: "2026-04-02", RewardAmount: 0.010},
			{CheckinDate: "2026-04-01", RewardAmount: 0.020},
		},
		totalCount:  12,
		totalReward: 0.345,
	}
	svc := NewCheckinService(repo, newCheckinSettings(true, "0.002", "0.020"), nil, nil)

	status, err := svc.GetStatus(context.Background(), 42, "2026-04", "Asia/Shanghai")

	require.NoError(t, err)
	require.True(t, status.Enabled)
	require.Equal(t, 0.002, status.MinReward)
	require.Equal(t, 0.020, status.MaxReward)
	require.True(t, status.Stats.CheckedInToday)
	require.Equal(t, int64(12), status.Stats.TotalCheckins)
	require.Equal(t, 0.345, status.Stats.TotalReward)
	require.Len(t, status.Stats.Records, 2)
	require.Equal(t, 2, status.Stats.CheckinCount)
	require.Equal(t, "2026-04-02", status.Stats.Records[0].CheckinDate)
	require.Equal(t, 0.010, status.Stats.Records[0].RewardAmount)
}
