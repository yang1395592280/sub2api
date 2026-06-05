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
	defaultCheckinMinRewardPoints       = int64(2)
	defaultCheckinMaxRewardPoints       = int64(20)
	defaultCheckinLuckyBonusSuccessRate = 50.0
)

var (
	ErrCheckinDisabled                  = infraerrors.Forbidden("CHECKIN_DISABLED", "check-in feature is disabled")
	ErrCheckinAlreadyToday              = infraerrors.Conflict("CHECKIN_ALREADY_TODAY", "already checked in today")
	ErrCheckinLuckyBonusDisabled        = infraerrors.Forbidden("CHECKIN_LUCKY_BONUS_DISABLED", "check-in lucky bonus is disabled")
	ErrCheckinLuckyBonusRequiresCheckin = infraerrors.Conflict("CHECKIN_LUCKY_BONUS_REQUIRES_CHECKIN", "check in today before using the lucky bonus")
	ErrCheckinLuckyBonusAlreadyPlayed   = infraerrors.Conflict("CHECKIN_LUCKY_BONUS_ALREADY_PLAYED", "lucky bonus already played today")
)

const (
	CheckinBonusStatusNone = "none"
	CheckinBonusStatusWin  = "win"
	CheckinBonusStatusLose = "lose"
)

type CheckinRecord struct {
	ID               int64
	UserID           int64
	CheckinDate      string
	RewardPoints     int64
	BaseRewardPoints int64
	BonusStatus      string
	BonusDeltaPoints int64
	UserTimezone     string
	CreatedAt        time.Time
	BonusPlayedAt    *time.Time
}

type CheckinRecordSummary struct {
	CheckinDate      string `json:"checkin_date"`
	RewardPoints     int64  `json:"reward_points"`
	BaseRewardPoints int64  `json:"base_reward_points"`
	BonusStatus      string `json:"bonus_status"`
	BonusDeltaPoints int64  `json:"bonus_delta_points"`
}

type CheckinTodayRecord struct {
	CheckinDate      string `json:"checkin_date"`
	RewardPoints     int64  `json:"reward_points"`
	BaseRewardPoints int64  `json:"base_reward_points"`
	BonusStatus      string `json:"bonus_status"`
	BonusDeltaPoints int64  `json:"bonus_delta_points"`
}

type CheckinStats struct {
	TotalRewardPoints int64                  `json:"total_reward_points"`
	TotalCheckins     int64                  `json:"total_checkins"`
	CheckinCount      int                    `json:"checkin_count"`
	CheckedInToday    bool                   `json:"checked_in_today"`
	Records           []CheckinRecordSummary `json:"records"`
}

type CheckinStatus struct {
	Enabled          bool                `json:"enabled"`
	MinRewardPoints  int64               `json:"min_reward_points"`
	MaxRewardPoints  int64               `json:"max_reward_points"`
	BonusEnabled     bool                `json:"bonus_enabled"`
	BonusAvailable   bool                `json:"bonus_available"`
	BonusSuccessRate float64             `json:"bonus_success_rate"`
	TodayRecord      *CheckinTodayRecord `json:"today_record,omitempty"`
	Stats            CheckinStats        `json:"stats"`
}

type AdminCheckinRecord struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	CheckinDate  string    `json:"checkin_date"`
	RewardPoints int64     `json:"reward_points"`
	UserTimezone string    `json:"user_timezone"`
	CreatedAt    time.Time `json:"created_at"`
}

type AdminCheckinAnalyticsFilter struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Search    string `json:"search"`
	Timezone  string `json:"timezone"`
	TopLimit  int    `json:"top_limit"`
}

type AdminCheckinOverview struct {
	TotalCheckins     int64 `json:"total_checkins"`
	TotalRewardPoints int64 `json:"total_reward_points"`
	TodayCheckins     int64 `json:"today_checkins"`
	AvgRewardPoints   int64 `json:"avg_reward_points"`
}

type AdminCheckinTrendPoint struct {
	Date         string `json:"date"`
	CheckinCount int64  `json:"checkin_count"`
	RewardPoints int64  `json:"reward_points"`
}

type AdminCheckinRewardBucket struct {
	Label        string `json:"label"`
	Min          int64  `json:"min"`
	Max          int64  `json:"max"`
	Count        int64  `json:"count"`
	RewardPoints int64  `json:"reward_points"`
}

type AdminCheckinTopUser struct {
	UserID            int64  `json:"user_id"`
	Email             string `json:"email"`
	Username          string `json:"username"`
	TotalCheckins     int64  `json:"total_checkins"`
	TotalRewardPoints int64  `json:"total_reward_points"`
}

type CheckinRepository interface {
	HasCheckedInOnDate(ctx context.Context, userID int64, date string) (bool, error)
	CreateAndCredit(ctx context.Context, record *CheckinRecord) (*CheckinRecord, error)
	ListByUserAndDateRange(ctx context.Context, userID int64, startDate, endDate string) ([]CheckinRecord, error)
	GetByUserAndDate(ctx context.Context, userID int64, date string) (*CheckinRecord, error)
	ApplyBonusOutcome(ctx context.Context, userID int64, date, outcome string, deltaPoints int64) (*CheckinRecord, error)
	GetUserTotals(ctx context.Context, userID int64) (int64, int64, error)
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
	randSource           *rand.Rand
	randMu               sync.Mutex
}

type checkinSettings struct {
	Enabled               bool
	MinRewardPoints       int64
	MaxRewardPoints       int64
	Distribution          []CheckinDistributionBucket
	LuckyBonusEnabled     bool
	LuckyBonusSuccessRate float64
}

func NewCheckinService(
	repo CheckinRepository,
	settingRepo SettingRepository,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	_ BillingCache,
) *CheckinService {
	return &CheckinService{
		repo:                 repo,
		settingRepo:          settingRepo,
		authCacheInvalidator: authCacheInvalidator,
		randSource:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *CheckinService) Checkin(ctx context.Context, userID int64, userTZ string) (*CheckinRecord, error) {
	settings := s.loadSettings(ctx)
	if !settings.Enabled {
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

	rewardPoints := s.randomRewardPoints(settings.MinRewardPoints, settings.MaxRewardPoints, settings.Distribution)
	record, err := s.repo.CreateAndCredit(ctx, &CheckinRecord{
		UserID:           userID,
		CheckinDate:      today,
		RewardPoints:     rewardPoints,
		BaseRewardPoints: rewardPoints,
		BonusStatus:      CheckinBonusStatusNone,
		UserTimezone:     userTZ,
	})
	if err != nil {
		return nil, err
	}
	record.BaseRewardPoints = record.RewardPoints

	s.invalidateCaches(ctx, userID)
	return record, nil
}

func (s *CheckinService) GetStatus(ctx context.Context, userID int64, month, userTZ string) (*CheckinStatus, error) {
	settings := s.loadSettings(ctx)
	status := &CheckinStatus{
		Enabled:          settings.Enabled,
		MinRewardPoints:  settings.MinRewardPoints,
		MaxRewardPoints:  settings.MaxRewardPoints,
		BonusEnabled:     settings.LuckyBonusEnabled,
		BonusSuccessRate: settings.LuckyBonusSuccessRate,
		Stats: CheckinStats{
			Records: []CheckinRecordSummary{},
		},
	}
	if !settings.Enabled {
		return status, nil
	}

	startDate, endDate := resolveMonthDateRange(month, userTZ)
	records, err := s.repo.ListByUserAndDateRange(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	totalCheckins, totalRewardPoints, err := s.repo.GetUserTotals(ctx, userID)
	if err != nil {
		return nil, err
	}
	today := timezoneutil.NowInUserLocation(userTZ).Format("2006-01-02")
	checkedToday, err := s.repo.HasCheckedInOnDate(ctx, userID, today)
	if err != nil {
		return nil, err
	}

	summaries := make([]CheckinRecordSummary, 0, len(records))
	for _, record := range records {
		summaries = append(summaries, CheckinRecordSummary{
			CheckinDate:      record.CheckinDate,
			RewardPoints:     record.RewardPoints,
			BaseRewardPoints: normalizeBaseRewardPoints(record),
			BonusStatus:      normalizeBonusStatus(record.BonusStatus),
			BonusDeltaPoints: record.BonusDeltaPoints,
		})
	}

	if checkedToday {
		todayRecord, err := s.repo.GetByUserAndDate(ctx, userID, today)
		if err != nil {
			return nil, err
		}
		status.TodayRecord = toTodayRecord(todayRecord)
		if todayRecord != nil && normalizeBonusStatus(todayRecord.BonusStatus) == CheckinBonusStatusNone {
			status.BonusAvailable = settings.LuckyBonusEnabled
		}
	}

	status.Stats = CheckinStats{
		TotalRewardPoints: totalRewardPoints,
		TotalCheckins:     totalCheckins,
		CheckinCount:      len(summaries),
		CheckedInToday:    checkedToday,
		Records:           summaries,
	}
	return status, nil
}

func (s *CheckinService) PlayLuckyBonus(ctx context.Context, userID int64, userTZ string) (*CheckinRecord, error) {
	settings := s.loadSettings(ctx)
	if !settings.Enabled || !settings.LuckyBonusEnabled {
		return nil, ErrCheckinLuckyBonusDisabled
	}

	today := timezoneutil.NowInUserLocation(userTZ).Format("2006-01-02")
	record, err := s.repo.GetByUserAndDate(ctx, userID, today)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrCheckinLuckyBonusRequiresCheckin
	}
	if normalizeBonusStatus(record.BonusStatus) != CheckinBonusStatusNone {
		return nil, ErrCheckinLuckyBonusAlreadyPlayed
	}

	baseRewardPoints := normalizeBaseRewardPoints(*record)
	finalRewardPoints := baseRewardPoints / 2
	outcome := CheckinBonusStatusLose
	if s.rollLuckyBonus(settings.LuckyBonusSuccessRate) {
		finalRewardPoints = baseRewardPoints * 2
		outcome = CheckinBonusStatusWin
	}
	deltaPoints := finalRewardPoints - record.RewardPoints

	updatedRecord, err := s.repo.ApplyBonusOutcome(ctx, userID, today, outcome, deltaPoints)
	if err != nil {
		return nil, err
	}

	s.invalidateCaches(ctx, userID)
	return updatedRecord, nil
}

func (s *CheckinService) loadSettings(ctx context.Context) checkinSettings {
	settings := checkinSettings{
		Enabled:               s.readBoolSetting(ctx, SettingKeyCheckinEnabled, false),
		MinRewardPoints:       s.readPointsSetting(ctx, SettingKeyCheckinMinReward, defaultCheckinMinRewardPoints),
		MaxRewardPoints:       s.readPointsSetting(ctx, SettingKeyCheckinMaxReward, defaultCheckinMaxRewardPoints),
		Distribution:          s.readDistributionSetting(ctx),
		LuckyBonusEnabled:     s.readBoolSetting(ctx, SettingKeyCheckinLuckyBonusEnabled, false),
		LuckyBonusSuccessRate: s.readRangedFloatSetting(ctx, SettingKeyCheckinLuckyBonusSuccessRate, defaultCheckinLuckyBonusSuccessRate, 0, 100),
	}
	if settings.MaxRewardPoints < settings.MinRewardPoints {
		settings.MaxRewardPoints = settings.MinRewardPoints
	}
	return settings
}

func (s *CheckinService) readBoolSetting(ctx context.Context, key string, fallback bool) bool {
	value, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	return value == "true"
}

func (s *CheckinService) readPointsSetting(ctx context.Context, key string, fallback int64) int64 {
	value, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return fallback
	}
	return int64(math.Round(parsed))
}

func (s *CheckinService) readRangedFloatSetting(ctx context.Context, key string, fallback, minValue, maxValue float64) float64 {
	value, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < minValue || parsed > maxValue {
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

func (s *CheckinService) randomRewardPoints(minRewardPoints, maxRewardPoints int64, distribution []CheckinDistributionBucket) int64 {
	if maxRewardPoints <= minRewardPoints {
		return minRewardPoints
	}
	s.randMu.Lock()
	defer s.randMu.Unlock()

	if len(distribution) > 0 {
		bucket := pickDistributionBucket(distribution, s.randSource.Float64())
		return rewardPointsFromDistribution(minRewardPoints, maxRewardPoints, bucket, s.randSource.Float64())
	}

	return minRewardPoints + s.randSource.Int63n(maxRewardPoints-minRewardPoints+1)
}

func (s *CheckinService) rollLuckyBonus(successRate float64) bool {
	if successRate <= 0 {
		return false
	}
	if successRate >= 100 {
		return true
	}

	s.randMu.Lock()
	defer s.randMu.Unlock()
	return s.randSource.Float64()*100 < successRate
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

func rewardPointsFromDistribution(minRewardPoints, maxRewardPoints int64, bucket CheckinDistributionBucket, roll float64) int64 {
	span := float64(maxRewardPoints - minRewardPoints)
	start := float64(minRewardPoints) + span*(bucket.StartPercent/100.0)
	end := float64(minRewardPoints) + span*(bucket.EndPercent/100.0)
	startPoints := int64(math.Round(start))
	endPoints := int64(math.Round(end))
	if endPoints < startPoints {
		endPoints = startPoints
	}
	if roll <= 0 {
		return startPoints
	}
	if roll >= 1 {
		return endPoints
	}
	width := endPoints - startPoints + 1
	if width <= 1 {
		return startPoints
	}
	offset := int64(math.Floor(roll * float64(width)))
	if offset >= width {
		offset = width - 1
	}
	return startPoints + offset
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

func normalizeBaseRewardPoints(record CheckinRecord) int64 {
	if record.BaseRewardPoints > 0 {
		return record.BaseRewardPoints
	}
	return record.RewardPoints
}

func normalizeBonusStatus(status string) string {
	switch status {
	case CheckinBonusStatusWin, CheckinBonusStatusLose:
		return status
	default:
		return CheckinBonusStatusNone
	}
}

func toTodayRecord(record *CheckinRecord) *CheckinTodayRecord {
	if record == nil {
		return nil
	}
	return &CheckinTodayRecord{
		CheckinDate:      record.CheckinDate,
		RewardPoints:     record.RewardPoints,
		BaseRewardPoints: normalizeBaseRewardPoints(*record),
		BonusStatus:      normalizeBonusStatus(record.BonusStatus),
		BonusDeltaPoints: record.BonusDeltaPoints,
	}
}

func (s *CheckinService) invalidateCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
}
