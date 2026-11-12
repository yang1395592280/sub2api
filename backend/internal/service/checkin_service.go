package service

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/shopspring/decimal"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	timezoneutil "github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	defaultCheckinMinReward             = 0.002
	defaultCheckinMaxReward             = 0.020
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
	ID               int64      `json:"id"`
	UserID           int64      `json:"user_id"`
	CheckinDate      string     `json:"checkin_date"`
	RewardAmount     float64    `json:"reward_amount"`
	BaseRewardAmount float64    `json:"base_reward_amount"`
	BonusStatus      string     `json:"bonus_status"`
	BonusDeltaAmount float64    `json:"bonus_delta_amount"`
	UserTimezone     string     `json:"user_timezone"`
	CreatedAt        time.Time  `json:"created_at"`
	BonusPlayedAt    *time.Time `json:"bonus_played_at"`
}

type CheckinRecordSummary struct {
	CheckinDate      string  `json:"checkin_date"`
	RewardAmount     float64 `json:"reward_amount"`
	BaseRewardAmount float64 `json:"base_reward_amount"`
	BonusStatus      string  `json:"bonus_status"`
	BonusDeltaAmount float64 `json:"bonus_delta_amount"`
}

type CheckinTodayRecord struct {
	CheckinDate      string  `json:"checkin_date"`
	RewardAmount     float64 `json:"reward_amount"`
	BaseRewardAmount float64 `json:"base_reward_amount"`
	BonusStatus      string  `json:"bonus_status"`
	BonusDeltaAmount float64 `json:"bonus_delta_amount"`
}

type CheckinStats struct {
	TotalReward    float64                `json:"total_reward"`
	TotalCheckins  int64                  `json:"total_checkins"`
	CheckinCount   int                    `json:"checkin_count"`
	CheckedInToday bool                   `json:"checked_in_today"`
	Records        []CheckinRecordSummary `json:"records"`
}

type CheckinStatus struct {
	Risk             *CheckinRiskNotice  `json:"risk,omitempty"`
	Timezone         string              `json:"timezone"`
	Enabled          bool                `json:"enabled"`
	MinReward        float64             `json:"min_reward"`
	MaxReward        float64             `json:"max_reward"`
	BonusEnabled     bool                `json:"bonus_enabled"`
	BonusAvailable   bool                `json:"bonus_available"`
	BonusSuccessRate float64             `json:"bonus_success_rate"`
	TodayRecord      *CheckinTodayRecord `json:"today_record,omitempty"`
	Stats            CheckinStats        `json:"stats"`
}

type CheckinRepository interface {
	HasCheckedInOnDate(ctx context.Context, userID int64, date string) (bool, error)
	CreateAndCredit(ctx context.Context, record *CheckinRecord) (*CheckinRecord, error)
	ListByUserAndDateRange(ctx context.Context, userID int64, startDate, endDate string) ([]CheckinRecord, error)
	GetByUserAndDate(ctx context.Context, userID int64, date string) (*CheckinRecord, error)
	ApplyBonusOutcome(ctx context.Context, userID int64, date, outcome string, delta float64) (*CheckinRecord, error)
	GetUserTotals(ctx context.Context, userID int64) (int64, float64, error)
	ListAdminRecords(ctx context.Context, page, pageSize int, search, date, timezone, sortBy, sortOrder string) ([]AdminCheckinRecord, int64, error)
	GetAdminOverview(ctx context.Context, filter AdminCheckinAnalyticsFilter) (AdminCheckinOverview, error)
	GetAdminTrend(ctx context.Context, filter AdminCheckinAnalyticsFilter) ([]AdminCheckinTrendPoint, error)
	GetAdminRewardDistribution(ctx context.Context, filter AdminCheckinAnalyticsFilter) ([]AdminCheckinRewardBucket, error)
	GetAdminTopUsers(ctx context.Context, filter AdminCheckinAnalyticsFilter) ([]AdminCheckinTopUser, error)
}

type CheckinService struct {
	riskStop             chan struct{}
	riskDone             chan struct{}
	riskStopOnce         sync.Once
	repo                 CheckinRepository
	settingRepo          SettingRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCache         BillingCache
	randSource           *rand.Rand
	randMu               sync.Mutex
}

type checkinSettings struct {
	Enabled               bool
	MinReward             float64
	MaxReward             float64
	Distribution          []CheckinDistributionBucket
	LuckyBonusEnabled     bool
	LuckyBonusSuccessRate float64
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
		randSource:           rand.New(secureCheckinSource{}),
	}
}

func (s *CheckinService) Checkin(ctx context.Context, userID int64, userTZ string) (*CheckinRecord, error) {
	settings := s.loadSettings(ctx)
	if !settings.Enabled {
		return nil, ErrCheckinDisabled
	}

	today := timezoneutil.NowInUserLocation("Asia/Shanghai").Format("2006-01-02")
	hasChecked, err := s.repo.HasCheckedInOnDate(ctx, userID, today)
	if err != nil {
		return nil, err
	}
	if hasChecked {
		return nil, ErrCheckinAlreadyToday
	}

	rewardAmount := s.randomReward(settings.MinReward, settings.MaxReward, settings.Distribution)
	record, err := s.repo.CreateAndCredit(ctx, &CheckinRecord{
		UserID:           userID,
		CheckinDate:      today,
		RewardAmount:     rewardAmount,
		BaseRewardAmount: rewardAmount,
		BonusStatus:      CheckinBonusStatusNone,
		UserTimezone:     userTZ,
	})
	if err != nil {
		return nil, err
	}
	record.BaseRewardAmount = record.RewardAmount

	s.invalidateCaches(ctx, userID)
	return record, nil
}

func (s *CheckinService) GetStatus(ctx context.Context, userID int64, month, userTZ string) (*CheckinStatus, error) {
	settings := s.loadSettings(ctx)
	status := &CheckinStatus{
		Enabled:          settings.Enabled,
		MinReward:        settings.MinReward,
		MaxReward:        settings.MaxReward,
		BonusEnabled:     settings.LuckyBonusEnabled,
		BonusSuccessRate: settings.LuckyBonusSuccessRate,
		Stats: CheckinStats{
			Records: []CheckinRecordSummary{},
		},
	}
	status.Timezone = "Asia/Shanghai"
	risk, err := s.Risk(ctx, userID)
	if err != nil {
		return nil, err
	}
	if risk != nil && risk.Status == "cleared" {
		status.Risk = &CheckinRiskNotice{CaseID: risk.ID, Reason: risk.Reason, ClearedAmount: risk.ClearedAmount, DecidedAt: risk.DecidedAt}
		status.Enabled = false
	}

	if !settings.Enabled && status.Risk == nil {
		return status, nil
	}

	startDate, endDate := resolveMonthDateRange(month, "Asia/Shanghai")
	records, err := s.repo.ListByUserAndDateRange(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	totalCheckins, totalReward, err := s.repo.GetUserTotals(ctx, userID)
	if err != nil {
		return nil, err
	}
	today := timezoneutil.NowInUserLocation("Asia/Shanghai").Format("2006-01-02")
	checkedToday, err := s.repo.HasCheckedInOnDate(ctx, userID, today)
	if err != nil {
		return nil, err
	}

	summaries := make([]CheckinRecordSummary, 0, len(records))
	for _, record := range records {
		summaries = append(summaries, CheckinRecordSummary{
			CheckinDate:      record.CheckinDate,
			RewardAmount:     record.RewardAmount,
			BaseRewardAmount: normalizeBaseReward(record),
			BonusStatus:      normalizeBonusStatus(record.BonusStatus),
			BonusDeltaAmount: record.BonusDeltaAmount,
		})
	}

	if checkedToday {
		todayRecord, err := s.repo.GetByUserAndDate(ctx, userID, today)
		if err != nil {
			return nil, err
		}
		status.TodayRecord = toTodayRecord(todayRecord)
		if todayRecord != nil && normalizeBonusStatus(todayRecord.BonusStatus) == CheckinBonusStatusNone {
			status.BonusAvailable = settings.LuckyBonusEnabled && status.Enabled
		}
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

func (s *CheckinService) PlayLuckyBonus(ctx context.Context, userID int64, userTZ string) (*CheckinRecord, error) {
	settings := s.loadSettings(ctx)
	if !settings.Enabled || !settings.LuckyBonusEnabled {
		return nil, ErrCheckinLuckyBonusDisabled
	}

	today := timezoneutil.NowInUserLocation("Asia/Shanghai").Format("2006-01-02")
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

	baseReward := normalizeBaseReward(*record)
	outcome := CheckinBonusStatusLose
	if s.rollLuckyBonus(settings.LuckyBonusSuccessRate) {
		outcome = CheckinBonusStatusWin
	}
	delta, err := CheckinBonusDelta(baseReward, record.RewardAmount, outcome)
	if err != nil {
		return nil, err
	}

	updatedRecord, err := s.repo.ApplyBonusOutcome(ctx, userID, today, outcome, delta)
	if err != nil {
		return nil, err
	}

	s.invalidateCaches(ctx, userID)
	return updatedRecord, nil
}

// CheckinBonusDelta shares decimal rounding with the locked ledger validation.
// A half-unit loss must round identically before and after persistence.
func CheckinBonusDelta(base, current float64, outcome string) (float64, error) {
	for _, amount := range []float64{base, current} {
		if math.IsNaN(amount) || math.IsInf(amount, 0) {
			return 0, infraerrors.BadRequest("CHECKIN_INVALID_BONUS", "invalid check-in reward")
		}
	}
	if base < 0 || base > 10000 {
		return 0, infraerrors.BadRequest("CHECKIN_INVALID_BONUS", "invalid check-in reward")
	}
	factor := decimal.NewFromInt(2)
	switch outcome {
	case CheckinBonusStatusLose:
		factor = decimal.NewFromFloat(-0.5)
	case CheckinBonusStatusWin:
	default:
		return 0, infraerrors.BadRequest("CHECKIN_INVALID_BONUS", "invalid check-in outcome")
	}
	return decimal.NewFromFloat(base).Mul(factor).Round(UsageBillingMonetaryScale).Sub(decimal.NewFromFloat(current)).Round(UsageBillingMonetaryScale).InexactFloat64(), nil
}

func (s *CheckinService) loadSettings(ctx context.Context) checkinSettings {
	settings := checkinSettings{
		Enabled:               s.readBoolSetting(ctx, SettingKeyCheckinEnabled, false),
		MinReward:             s.readFloatSetting(ctx, SettingKeyCheckinMinReward, defaultCheckinMinReward),
		MaxReward:             s.readFloatSetting(ctx, SettingKeyCheckinMaxReward, defaultCheckinMaxReward),
		Distribution:          s.readDistributionSetting(ctx),
		LuckyBonusEnabled:     s.readBoolSetting(ctx, SettingKeyCheckinLuckyBonusEnabled, false),
		LuckyBonusSuccessRate: s.readRangedFloatSetting(ctx, SettingKeyCheckinLuckyBonusSuccessRate, defaultCheckinLuckyBonusSuccessRate, 0, 100),
	}
	if settings.MaxReward < settings.MinReward {
		settings.MaxReward = settings.MinReward
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

func (s *CheckinService) readFloatSetting(ctx context.Context, key string, fallback float64) float64 {
	value, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return fallback
	}
	return parsed
}

func (s *CheckinService) readRangedFloatSetting(ctx context.Context, key string, fallback, minValue, maxValue float64) float64 {
	value := s.readFloatSetting(ctx, key, fallback)
	if value < minValue || value > maxValue {
		return fallback
	}
	return value
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
	base := timezoneutil.NowInUserLocation("Asia/Shanghai")
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
	return QuantizeUsageBillingAmount(value)
}

func normalizeBaseReward(record CheckinRecord) float64 {
	if record.BaseRewardAmount > 0 {
		return record.BaseRewardAmount
	}
	return record.RewardAmount
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
		RewardAmount:     record.RewardAmount,
		BaseRewardAmount: normalizeBaseReward(*record),
		BonusStatus:      normalizeBonusStatus(record.BonusStatus),
		BonusDeltaAmount: record.BonusDeltaAmount,
	}
}

func (s *CheckinService) invalidateCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCache == nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.billingCache.InvalidateUserBalance(cacheCtx, userID); err != nil {
		logger.LegacyPrintf("service.checkin", "invalidate balance cache user=%d: %v", userID, err)
	}
}

// Use OS randomness for monetary outcomes; tests may inject a seeded source.
type secureCheckinSource struct{}

func (secureCheckinSource) Seed(int64)   {}
func (secureCheckinSource) Int63() int64 { return int64(secureCheckinSource{}.Uint64() >> 1) }
func (secureCheckinSource) Uint64() uint64 {
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint64(b[:])
}
