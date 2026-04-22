package service

import (
	"context"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	timezoneutil "github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	defaultCheckinMinReward = 0.002
	defaultCheckinMaxReward = 0.020
)

var (
	ErrCheckinDisabled     = infraerrors.Forbidden("CHECKIN_DISABLED", "check-in feature is disabled")
	ErrCheckinAlreadyToday = infraerrors.Conflict("CHECKIN_ALREADY_TODAY", "already checked in today")
)

type CheckinRecord struct {
	ID           int64
	UserID       int64
	CheckinDate  string
	RewardAmount float64
	UserTimezone string
	CreatedAt    time.Time
}

type CheckinRecordSummary struct {
	CheckinDate  string  `json:"checkin_date"`
	RewardAmount float64 `json:"reward_amount"`
}

type CheckinStats struct {
	TotalReward    float64                `json:"total_reward"`
	TotalCheckins  int64                  `json:"total_checkins"`
	CheckinCount   int                    `json:"checkin_count"`
	CheckedInToday bool                   `json:"checked_in_today"`
	Records        []CheckinRecordSummary `json:"records"`
}

type CheckinStatus struct {
	Enabled   bool         `json:"enabled"`
	MinReward float64      `json:"min_reward"`
	MaxReward float64      `json:"max_reward"`
	Stats     CheckinStats `json:"stats"`
}

type CheckinRepository interface {
	HasCheckedInOnDate(ctx context.Context, userID int64, date string) (bool, error)
	CreateAndCredit(ctx context.Context, record *CheckinRecord) (*CheckinRecord, error)
	ListByUserAndDateRange(ctx context.Context, userID int64, startDate, endDate string) ([]CheckinRecord, error)
	GetUserTotals(ctx context.Context, userID int64) (int64, float64, error)
	ListAdminRecords(ctx context.Context, page, pageSize int, search, date, timezone, sortBy, sortOrder string) ([]AdminCheckinRecord, int64, error)
	GetAdminOverview(ctx context.Context, filter AdminCheckinAnalyticsFilter) (AdminCheckinOverview, error)
	GetAdminTrend(ctx context.Context, filter AdminCheckinAnalyticsFilter) ([]AdminCheckinTrendPoint, error)
	GetAdminRewardDistribution(ctx context.Context, filter AdminCheckinAnalyticsFilter) ([]AdminCheckinRewardBucket, error)
	GetAdminTopUsers(ctx context.Context, filter AdminCheckinAnalyticsFilter) ([]AdminCheckinTopUser, error)
}

type CheckinService struct {
	repo                 CheckinRepository
	settingRepo          SettingRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCache         BillingCache
	randSource           *rand.Rand
	randMu               sync.Mutex
}

func NewCheckinService(
	repo CheckinRepository,
	settingRepo SettingRepository,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCache BillingCache,
) *CheckinService {
	return &CheckinService{
		repo:                 repo,
		settingRepo:          settingRepo,
		authCacheInvalidator: authCacheInvalidator,
		billingCache:         billingCache,
		randSource:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *CheckinService) Checkin(ctx context.Context, userID int64, userTZ string) (*CheckinRecord, error) {
	enabled, minReward, maxReward, distribution := s.loadSettings(ctx)
	if !enabled {
		return nil, ErrCheckinDisabled
	}

	today := timezoneutil.NowInUserLocation(userTZ).Format("2006-01-02")
	hasChecked, err := s.repo.HasCheckedInOnDate(ctx, userID, today)
	if err != nil {
		return nil, err
	}
	if hasChecked {
		return nil, ErrCheckinAlreadyToday
	}

	record, err := s.repo.CreateAndCredit(ctx, &CheckinRecord{
		UserID:       userID,
		CheckinDate:  today,
		RewardAmount: s.randomReward(minReward, maxReward, distribution),
		UserTimezone: userTZ,
	})
	if err != nil {
		return nil, err
	}

	s.invalidateCaches(ctx, userID)
	return record, nil
}

func (s *CheckinService) GetStatus(ctx context.Context, userID int64, month, userTZ string) (*CheckinStatus, error) {
	enabled, minReward, maxReward, _ := s.loadSettings(ctx)
	status := &CheckinStatus{
		Enabled:   enabled,
		MinReward: minReward,
		MaxReward: maxReward,
		Stats: CheckinStats{
			Records: []CheckinRecordSummary{},
		},
	}
	if !enabled {
		return status, nil
	}

	startDate, endDate := resolveMonthDateRange(month, userTZ)
	records, err := s.repo.ListByUserAndDateRange(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	totalCheckins, totalReward, err := s.repo.GetUserTotals(ctx, userID)
	if err != nil {
		return nil, err
	}
	checkedToday, err := s.repo.HasCheckedInOnDate(ctx, userID, timezoneutil.NowInUserLocation(userTZ).Format("2006-01-02"))
	if err != nil {
		return nil, err
	}

	summaries := make([]CheckinRecordSummary, 0, len(records))
	for _, record := range records {
		summaries = append(summaries, CheckinRecordSummary{
			CheckinDate:  record.CheckinDate,
			RewardAmount: record.RewardAmount,
		})
	}

	status.Stats = CheckinStats{
		TotalReward:    totalReward,
		TotalCheckins:  totalCheckins,
		CheckinCount:   len(summaries),
		CheckedInToday: checkedToday,
		Records:        summaries,
	}
	return status, nil
}

func (s *CheckinService) loadSettings(ctx context.Context) (bool, float64, float64, []CheckinDistributionBucket) {
	enabled := s.readBoolSetting(ctx, SettingKeyCheckinEnabled, false)
	minReward := s.readFloatSetting(ctx, SettingKeyCheckinMinReward, defaultCheckinMinReward)
	maxReward := s.readFloatSetting(ctx, SettingKeyCheckinMaxReward, defaultCheckinMaxReward)
	if maxReward < minReward {
		maxReward = minReward
	}
	distribution := s.readDistributionSetting(ctx)
	return enabled, minReward, maxReward, distribution
}

func (s *CheckinService) readBoolSetting(ctx context.Context, key string, fallback bool) bool {
	value, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	return value == "true"
}

func (s *CheckinService) readFloatSetting(ctx context.Context, key string, fallback float64) float64 {
	value, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func (s *CheckinService) readDistributionSetting(ctx context.Context) []CheckinDistributionBucket {
	if !s.readBoolSetting(ctx, SettingKeyCheckinDistributionEnabled, false) {
		return nil
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyCheckinDistributionConfig)
	if err != nil {
		return nil
	}
	buckets, err := ParseCheckinDistributionConfig(raw)
	if err != nil {
		return nil
	}
	return buckets
}

func (s *CheckinService) randomReward(minReward, maxReward float64, distribution []CheckinDistributionBucket) float64 {
	if maxReward <= minReward {
		return roundTo8(minReward)
	}
	s.randMu.Lock()
	defer s.randMu.Unlock()

	if len(distribution) > 0 {
		bucket := pickDistributionBucket(distribution, s.randSource.Float64())
		value := rewardFromDistribution(minReward, maxReward, bucket, s.randSource.Float64())
		return roundTo8(value)
	}

	value := minReward + s.randSource.Float64()*(maxReward-minReward)
	return roundTo8(value)
}

func pickDistributionBucket(buckets []CheckinDistributionBucket, roll float64) CheckinDistributionBucket {
	if len(buckets) == 0 {
		return CheckinDistributionBucket{StartPercent: 0, EndPercent: 100, Weight: 1}
	}

	totalWeight := 0
	for _, bucket := range buckets {
		totalWeight += bucket.Weight
	}
	if totalWeight <= 0 {
		return buckets[0]
	}

	target := roll * float64(totalWeight)
	current := 0.0
	for _, bucket := range buckets {
		current += float64(bucket.Weight)
		if target < current {
			return bucket
		}
	}

	return buckets[len(buckets)-1]
}

func rewardFromDistribution(minReward, maxReward float64, bucket CheckinDistributionBucket, roll float64) float64 {
	span := maxReward - minReward
	start := minReward + span*(bucket.StartPercent/100.0)
	end := minReward + span*(bucket.EndPercent/100.0)
	if end < start {
		end = start
	}
	if roll <= 0 {
		return roundTo8(start)
	}
	if roll >= 1 {
		return roundTo8(end)
	}
	return roundTo8(start + (end-start)*roll)
}

func resolveMonthDateRange(month, userTZ string) (string, string) {
	base := timezoneutil.NowInUserLocation(userTZ)
	if month != "" {
		if parsed, err := timezoneutil.ParseInUserLocation("2006-01", month, userTZ); err == nil {
			base = parsed
		}
	}
	loc := base.Location()
	start := time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, -1)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

func roundTo8(value float64) float64 {
	return math.Round(value*1e8) / 1e8
}

func (s *CheckinService) invalidateCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCache == nil {
		return
	}
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.billingCache.InvalidateUserBalance(cacheCtx, userID)
	}()
}
