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
	adminRecords     []AdminCheckinRecord
	adminTotal       int64
	adminErr         error
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

func (s *checkinRepoStub) ListAdminRecords(_ context.Context, page, pageSize int, search, date, sortBy, sortOrder string) ([]AdminCheckinRecord, int64, error) {
	if s.adminErr != nil {
		return nil, 0, s.adminErr
	}
	return s.adminRecords, s.adminTotal, nil
}

func newCheckinSettings(enabled bool, minReward, maxReward string) *checkinSettingRepoStub {
	return &checkinSettingRepoStub{
		values: map[string]string{
			SettingKeyCheckinEnabled:             map[bool]string{true: "true", false: "false"}[enabled],
			SettingKeyCheckinMinReward:           minReward,
			SettingKeyCheckinMaxReward:           maxReward,
			SettingKeyCheckinDistributionEnabled: "false",
			SettingKeyCheckinDistributionConfig:  "[]",
		},
	}
}

func newCheckinSettingsWithDistribution(enabled bool, minReward, maxReward, distribution string) *checkinSettingRepoStub {
	settings := newCheckinSettings(enabled, minReward, maxReward)
	settings.values[SettingKeyCheckinDistributionEnabled] = "true"
	settings.values[SettingKeyCheckinDistributionConfig] = distribution
	return settings
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

func TestPickDistributionBucketUsesWeightPriority(t *testing.T) {
	t.Parallel()

	buckets := []CheckinDistributionBucket{
		{StartPercent: 0, EndPercent: 25, Weight: 50},
		{StartPercent: 25, EndPercent: 50, Weight: 30},
		{StartPercent: 50, EndPercent: 75, Weight: 15},
		{StartPercent: 75, EndPercent: 90, Weight: 4},
		{StartPercent: 90, EndPercent: 100, Weight: 1},
	}

	require.Equal(t, buckets[0], pickDistributionBucket(buckets, 0.49))
	require.Equal(t, buckets[1], pickDistributionBucket(buckets, 0.79))
	require.Equal(t, buckets[2], pickDistributionBucket(buckets, 0.94))
	require.Equal(t, buckets[3], pickDistributionBucket(buckets, 0.98))
	require.Equal(t, buckets[4], pickDistributionBucket(buckets, 0.999))
}

func TestRewardFromDistributionMapsBucketToRange(t *testing.T) {
	t.Parallel()

	bucket := CheckinDistributionBucket{StartPercent: 25, EndPercent: 50, Weight: 30}

	reward := rewardFromDistribution(10, 100, bucket, 0)
	require.GreaterOrEqual(t, reward, 32.5)
	require.LessOrEqual(t, reward, 55.0)

	reward = rewardFromDistribution(10, 100, bucket, 1)
	require.Equal(t, 55.0, reward)
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

	record, err := svc.Checkin(context.Background(), 42, "Asia/Shanghai")

	require.NoError(t, err)
	require.NotNil(t, record)
	require.GreaterOrEqual(t, repo.lastCreateRecord.RewardAmount, 10.0)
	require.LessOrEqual(t, repo.lastCreateRecord.RewardAmount, 32.5)
}

func TestParseCheckinDistributionConfigRejectsGapBuckets(t *testing.T) {
	t.Parallel()

	_, err := ParseCheckinDistributionConfig(`[{"start_percent":0,"end_percent":20,"weight":5},{"start_percent":25,"end_percent":100,"weight":1}]`)

	require.Error(t, err)
	require.ErrorContains(t, err, "起点必须紧接上一档终点")
}

func TestParseCheckinDistributionConfigAcceptsFullCoverageBuckets(t *testing.T) {
	t.Parallel()

	buckets, err := ParseCheckinDistributionConfig(`[{"start_percent":0,"end_percent":25,"weight":50},{"start_percent":25,"end_percent":50,"weight":30},{"start_percent":50,"end_percent":75,"weight":15},{"start_percent":75,"end_percent":90,"weight":4},{"start_percent":90,"end_percent":100,"weight":1}]`)

	require.NoError(t, err)
	require.Len(t, buckets, 5)
	require.Equal(t, 50, buckets[0].Weight)
	require.Equal(t, 100.0, buckets[4].EndPercent)
}

func TestCheckinServiceListAdminRecordsReturnsPaginatedResults(t *testing.T) {
	t.Parallel()

	repo := &checkinRepoStub{
		adminRecords: []AdminCheckinRecord{
			{UserID: 1, UserEmail: "a@example.com", UserName: "alice", CheckinDate: "2026-04-21", RewardAmount: 12.5},
		},
		adminTotal: 1,
	}
	svc := NewCheckinService(repo, newCheckinSettings(true, "10", "100"), nil, nil)

	items, total, err := svc.ListAdminRecords(context.Background(), 1, 20, "alice", "2026-04-21", "created_at", "desc")

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "a@example.com", items[0].UserEmail)
	require.Equal(t, 12.5, items[0].RewardAmount)
}
