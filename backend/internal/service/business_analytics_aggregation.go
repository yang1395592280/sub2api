package service

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	businessAnalyticsAggregationJobName = "business_analytics:aggregation"
	defaultBusinessAnalyticsRunTimeout  = 2 * time.Minute
)

var (
	ErrBusinessAnalyticsRecomputeDisabled  = errors.New("经营分析聚合已禁用")
	ErrBusinessAnalyticsRecomputeTooLarge  = errors.New("经营分析聚合回填时间跨度过大")
	errBusinessAnalyticsAggregationRunning = errors.New("经营分析聚合作业正在运行")
)

type businessAnalyticsAggregationScheduler interface {
	ScheduleRecurring(name string, interval time.Duration, fn func())
	Cancel(name string)
}

// BusinessAnalyticsAggregationService 负责经营分析聚合表的定时重算。
type BusinessAnalyticsAggregationService struct {
	repo        BusinessAnalyticsAggregationRepository
	timingWheel businessAnalyticsAggregationScheduler
	cfg         config.BusinessAnalyticsConfig
	running     int32
}

func NewBusinessAnalyticsAggregationService(repo BusinessAnalyticsAggregationRepository, timingWheel *TimingWheelService, cfg *config.Config) *BusinessAnalyticsAggregationService {
	aggCfg := config.BusinessAnalyticsConfig{
		Enabled:                    true,
		AggregationIntervalSeconds: 300,
		LookbackSeconds:            7200,
		BackfillEnabled:            true,
		BackfillMaxDays:            90,
	}
	if cfg != nil {
		aggCfg = cfg.BusinessAnalytics
	}
	return &BusinessAnalyticsAggregationService{
		repo:        repo,
		timingWheel: timingWheel,
		cfg:         normalizeBusinessAnalyticsAggregationConfig(aggCfg),
	}
}

func ProvideBusinessAnalyticsAggregationService(repo BusinessAnalyticsAggregationRepository, timingWheel *TimingWheelService, cfg *config.Config) *BusinessAnalyticsAggregationService {
	svc := NewBusinessAnalyticsAggregationService(repo, timingWheel, cfg)
	svc.Start()
	return svc
}

func (s *BusinessAnalyticsAggregationService) Start() {
	if s == nil || s.repo == nil || s.timingWheel == nil {
		return
	}
	if !s.cfg.Enabled {
		logger.LegacyPrintf("service.business_analytics_aggregation", "%s", "[BusinessAnalyticsAggregation] 聚合作业已禁用")
		return
	}
	interval := time.Duration(s.cfg.AggregationIntervalSeconds) * time.Second
	s.timingWheel.ScheduleRecurring(businessAnalyticsAggregationJobName, interval, func() {
		s.runScheduledAggregation()
	})
	logger.LegacyPrintf("service.business_analytics_aggregation", "[BusinessAnalyticsAggregation] 聚合作业启动 (interval=%v lookback=%ds)", interval, s.cfg.LookbackSeconds)
}

func (s *BusinessAnalyticsAggregationService) Stop() {
	if s == nil || s.timingWheel == nil {
		return
	}
	s.timingWheel.Cancel(businessAnalyticsAggregationJobName)
}

func (s *BusinessAnalyticsAggregationService) TriggerRecomputeRange(start, end time.Time) error {
	if s == nil || s.repo == nil {
		return errors.New("经营分析聚合服务未初始化")
	}
	if !s.cfg.Enabled {
		return ErrBusinessAnalyticsRecomputeDisabled
	}
	if !s.cfg.BackfillEnabled {
		return ErrBusinessAnalyticsRecomputeDisabled
	}
	if !end.After(start) {
		return errors.New("经营分析聚合时间范围无效")
	}
	if s.cfg.BackfillMaxDays > 0 {
		maxRange := time.Duration(s.cfg.BackfillMaxDays) * 24 * time.Hour
		if end.Sub(start) > maxRange {
			return ErrBusinessAnalyticsRecomputeTooLarge
		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), defaultBusinessAnalyticsRunTimeout)
		defer cancel()
		if err := s.recomputeRange(ctx, start, end); err != nil {
			logger.LegacyPrintf("service.business_analytics_aggregation", "[BusinessAnalyticsAggregation] 手动重算失败: %v", err)
		}
	}()
	return nil
}

func (s *BusinessAnalyticsAggregationService) runScheduledAggregation() {
	if s == nil || s.repo == nil || !s.cfg.Enabled {
		return
	}
	now := time.Now().UTC()
	lookback := time.Duration(s.cfg.LookbackSeconds) * time.Second
	if lookback < 0 {
		lookback = 0
	}
	start := truncateToDayUTC(now.Add(-lookback))
	end := truncateToDayUTC(now).AddDate(0, 0, 1)
	ctx, cancel := context.WithTimeout(context.Background(), defaultBusinessAnalyticsRunTimeout)
	defer cancel()
	if err := s.recomputeRange(ctx, start, end); err != nil && !errors.Is(err, errBusinessAnalyticsAggregationRunning) {
		logger.LegacyPrintf("service.business_analytics_aggregation", "[BusinessAnalyticsAggregation] 定时聚合失败: %v", err)
	}
}

func (s *BusinessAnalyticsAggregationService) recomputeRange(ctx context.Context, start, end time.Time) error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return errBusinessAnalyticsAggregationRunning
	}
	defer atomic.StoreInt32(&s.running, 0)

	if err := s.repo.RecomputeDaily(ctx, start.UTC(), end.UTC()); err != nil {
		return err
	}
	for _, weekStart := range weekStartsInRangeUTC(start, end) {
		if err := s.repo.RecomputeWeekly(ctx, weekStart); err != nil {
			return err
		}
	}
	return nil
}

func normalizeBusinessAnalyticsAggregationConfig(cfg config.BusinessAnalyticsConfig) config.BusinessAnalyticsConfig {
	if cfg.AggregationIntervalSeconds <= 0 {
		cfg.AggregationIntervalSeconds = 300
	}
	if cfg.LookbackSeconds < 0 {
		cfg.LookbackSeconds = 7200
	}
	if cfg.BackfillMaxDays < 0 {
		cfg.BackfillMaxDays = 90
	}
	return cfg
}

func currentWeekStartUTC(t time.Time) time.Time {
	day := truncateToDayUTC(t)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func weekStartsInRangeUTC(start, end time.Time) []time.Time {
	if !end.After(start) {
		return nil
	}
	first := currentWeekStartUTC(start)
	last := currentWeekStartUTC(end.Add(-time.Nanosecond))
	weeks := make([]time.Time, 0, int(last.Sub(first)/(7*24*time.Hour))+1)
	for weekStart := first; !weekStart.After(last); weekStart = weekStart.AddDate(0, 0, 7) {
		weeks = append(weeks, weekStart)
	}
	return weeks
}
