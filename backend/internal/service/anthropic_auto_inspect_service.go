package service

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/robfig/cron/v3"
)

const (
	anthropicAutoInspectDefaultErrorCooldownMinutes = 30
	anthropicAutoInspectDefaultPerAccountTimeout    = 45 * time.Second
)

var anthropicAutoInspectResetAtPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`),
	regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`),
}

type anthropicAutoInspectAccountRepo interface {
	ListByPlatform(ctx context.Context, platform string) ([]Account, error)
	SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error
}

type anthropicAutoInspectTestRunner interface {
	RunTestBackground(ctx context.Context, accountID int64, modelID string) (*ScheduledTestResult, error)
}

type anthropicAutoInspectSettingsProvider interface {
	GetAllSettings(ctx context.Context) (*SystemSettings, error)
	UpdateSettings(ctx context.Context, settings *SystemSettings) error
}

type AnthropicAutoInspectRunInput struct {
	TriggerSource string
}

type AnthropicInspectClassification struct {
	Result  AnthropicAutoInspectResult
	ResetAt *time.Time
	Reason  string
}

type AnthropicAutoInspectService struct {
	accountRepo      anthropicAutoInspectAccountRepo
	testRunner       anthropicAutoInspectTestRunner
	repo             AnthropicAutoInspectRepository
	settingsProvider anthropicAutoInspectSettingsProvider
	cfg              *config.Config

	now               func() time.Time
	perAccountTimeout time.Duration
	running           atomic.Bool
	cron              *cron.Cron
	startOnce         sync.Once
	stopOnce          sync.Once
}

func NewAnthropicAutoInspectService(
	accountRepo anthropicAutoInspectAccountRepo,
	testRunner anthropicAutoInspectTestRunner,
	repo AnthropicAutoInspectRepository,
	cfg *config.Config,
) *AnthropicAutoInspectService {
	return &AnthropicAutoInspectService{
		accountRepo:       accountRepo,
		testRunner:        testRunner,
		repo:              repo,
		cfg:               cfg,
		now:               time.Now,
		perAccountTimeout: anthropicAutoInspectDefaultPerAccountTimeout,
	}
}

func (s *AnthropicAutoInspectService) SetSettingsProvider(provider anthropicAutoInspectSettingsProvider) {
	s.settingsProvider = provider
}

func (s *AnthropicAutoInspectService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}
		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc("* * * * *", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			if err := s.RunBatch(ctx, AnthropicAutoInspectRunInput{TriggerSource: AnthropicAutoInspectTriggerSourceScheduler}); err != nil {
				logger.LegacyPrintf("service.anthropic_auto_inspect", "[AnthropicAutoInspect] scheduled run failed: %v", err)
			}
		})
		if err != nil {
			logger.LegacyPrintf("service.anthropic_auto_inspect", "[AnthropicAutoInspect] not started: %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
	})
}

func (s *AnthropicAutoInspectService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			<-s.cron.Stop().Done()
		}
	})
}

func (s *AnthropicAutoInspectService) RunBatch(ctx context.Context, input AnthropicAutoInspectRunInput) error {
	if s == nil {
		return nil
	}
	if input.TriggerSource == AnthropicAutoInspectTriggerSourceScheduler && !s.schedulerEnabled(ctx) {
		return nil
	}
	if !s.running.CompareAndSwap(false, true) {
		now := s.now()
		if _, err := s.repo.CreateSkippedBatch(ctx, CreateAnthropicAutoInspectSkippedBatchInput{
			TriggerSource: input.TriggerSource,
			SkipReason:    "batch_already_running",
			StartedAt:     now,
			FinishedAt:    now,
		}); err != nil {
			logger.LegacyPrintf(
				"service.anthropic_auto_inspect",
				"[AnthropicAutoInspect] failed to persist skipped batch: source=%s err=%v",
				input.TriggerSource,
				err,
			)
		}
		logger.LegacyPrintf(
			"service.anthropic_auto_inspect",
			"[AnthropicAutoInspect] skipped run because previous batch still running: source=%s",
			input.TriggerSource,
		)
		return nil
	}
	defer s.running.Store(false)

	startedAt := s.now()
	batchID, err := s.repo.CreateBatch(ctx, CreateAnthropicAutoInspectBatchInput{
		TriggerSource: input.TriggerSource,
		Status:        AnthropicAutoInspectBatchStatusRunning,
		StartedAt:     startedAt,
	})
	if err != nil {
		return err
	}

	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformAnthropic)
	stats := AnthropicAutoInspectBatchStats{}
	if err != nil {
		_ = s.repo.MarkBatchFailed(ctx, batchID, stats, s.now())
		return err
	}

	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].ID < accounts[j].ID
	})

	stats.TotalAccounts = len(accounts)
	for _, account := range accounts {
		stats.ProcessedAccounts++
		if shouldSkipAnthropicAutoInspect(account, s.now()) {
			stats.SkippedCount++
			if err := s.repo.CreateLog(ctx, buildSkippedAnthropicAutoInspectLog(batchID, account, s.now())); err != nil {
				return s.failBatch(ctx, batchID, stats, err)
			}
			continue
		}

		result, logEntry, err := s.inspectOne(ctx, batchID, account)
		if err != nil {
			return s.failBatch(ctx, batchID, stats, err)
		}
		if err := s.repo.CreateLog(ctx, logEntry); err != nil {
			return s.failBatch(ctx, batchID, stats, err)
		}
		switch result {
		case AnthropicAutoInspectResultSuccess:
			stats.SuccessCount++
		case AnthropicAutoInspectResultRateLimited:
			stats.RateLimitedCount++
		case AnthropicAutoInspectResultError:
			stats.ErrorCount++
		case AnthropicAutoInspectResultSkipped:
			stats.SkippedCount++
		}
	}

	return s.repo.CompleteBatch(ctx, batchID, stats, s.now())
}

func (s *AnthropicAutoInspectService) failBatch(ctx context.Context, batchID int64, stats AnthropicAutoInspectBatchStats, cause error) error {
	if err := s.repo.MarkBatchFailed(ctx, batchID, stats, s.now()); err != nil {
		logger.LegacyPrintf(
			"service.anthropic_auto_inspect",
			"[AnthropicAutoInspect] failed to mark batch=%d failed after error: cause=%v mark_err=%v",
			batchID,
			cause,
			err,
		)
	}
	return cause
}

func (s *AnthropicAutoInspectService) inspectOne(ctx context.Context, batchID int64, account Account) (AnthropicAutoInspectResult, AnthropicAutoInspectLog, error) {
	accountCtx := ctx
	cancel := func() {}
	if s.perAccountTimeout > 0 {
		accountCtx, cancel = context.WithTimeout(ctx, s.perAccountTimeout)
	}
	defer cancel()

	now := s.now()
	result, err := s.testRunner.RunTestBackground(accountCtx, account.ID, "")
	if err != nil {
		result = &ScheduledTestResult{
			Status:       "failed",
			ErrorMessage: err.Error(),
			StartedAt:    now,
			FinishedAt:   s.now(),
		}
	}
	if result == nil {
		result = &ScheduledTestResult{
			Status:     "failed",
			StartedAt:  now,
			FinishedAt: s.now(),
		}
	}
	if result.StartedAt.IsZero() {
		result.StartedAt = now
	}
	if result.FinishedAt.IsZero() {
		result.FinishedAt = s.now()
		result.LatencyMs = result.FinishedAt.Sub(result.StartedAt).Milliseconds()
	}

	classification := classifyAnthropicAutoInspect(result)
	logEntry := AnthropicAutoInspectLog{
		BatchID:             batchID,
		AccountID:           account.ID,
		AccountNameSnapshot: account.Name,
		Platform:            account.Platform,
		AccountType:         account.Type,
		Result:              classification.Result,
		ResponseText:        result.ResponseText,
		ErrorMessage:        result.ErrorMessage,
		RateLimitResetAt:    classification.ResetAt,
		StartedAt:           result.StartedAt,
		FinishedAt:          result.FinishedAt,
		LatencyMs:           result.LatencyMs,
	}

	if classification.Result == AnthropicAutoInspectResultRateLimited || classification.Result == AnthropicAutoInspectResultError {
		until := s.cooldownUntil(classification.ResetAt)
		logEntry.TempUnschedulableUntil = &until
		logEntry.SchedulableChanged = true
		if err := s.accountRepo.SetTempUnschedulable(accountCtx, account.ID, until, classification.Reason); err != nil {
			return classification.Result, logEntry, err
		}
	}

	return classification.Result, logEntry, nil
}

func (s *AnthropicAutoInspectService) cooldownUntil(resetAt *time.Time) time.Time {
	if resetAt != nil && !resetAt.IsZero() {
		return resetAt.UTC()
	}
	return s.now().Add(time.Duration(s.errorCooldownMinutes()) * time.Minute)
}

func (s *AnthropicAutoInspectService) errorCooldownMinutes() int {
	if s.settingsProvider != nil {
		settings, err := s.settingsProvider.GetAllSettings(context.Background())
		if err == nil && settings != nil && settings.AnthropicAutoInspectErrorCooldownMinutes > 0 {
			return settings.AnthropicAutoInspectErrorCooldownMinutes
		}
	}
	return anthropicAutoInspectDefaultErrorCooldownMinutes
}

func (s *AnthropicAutoInspectService) schedulerEnabled(ctx context.Context) bool {
	if s.settingsProvider == nil {
		return false
	}
	settings, err := s.settingsProvider.GetAllSettings(ctx)
	return err == nil && settings != nil && settings.AnthropicAutoInspectEnabled
}

func (s *AnthropicAutoInspectService) GetSettings(ctx context.Context) (*AnthropicAutoInspectSettings, error) {
	if s.settingsProvider == nil {
		return &AnthropicAutoInspectSettings{
			Enabled:              false,
			IntervalMinutes:      1,
			ErrorCooldownMinutes: anthropicAutoInspectDefaultErrorCooldownMinutes,
		}, nil
	}
	settings, err := s.settingsProvider.GetAllSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &AnthropicAutoInspectSettings{
		Enabled:              settings.AnthropicAutoInspectEnabled,
		IntervalMinutes:      settings.AnthropicAutoInspectIntervalMinutes,
		ErrorCooldownMinutes: settings.AnthropicAutoInspectErrorCooldownMinutes,
	}, nil
}

func (s *AnthropicAutoInspectService) UpdateSettings(ctx context.Context, input AnthropicAutoInspectSettings) error {
	if s.settingsProvider == nil {
		return nil
	}
	settings, err := s.settingsProvider.GetAllSettings(ctx)
	if err != nil {
		return err
	}
	settings.AnthropicAutoInspectEnabled = input.Enabled
	settings.AnthropicAutoInspectIntervalMinutes = input.IntervalMinutes
	settings.AnthropicAutoInspectErrorCooldownMinutes = input.ErrorCooldownMinutes
	return s.settingsProvider.UpdateSettings(ctx, settings)
}

func (s *AnthropicAutoInspectService) RunNow(ctx context.Context) error {
	return s.RunBatch(ctx, AnthropicAutoInspectRunInput{TriggerSource: AnthropicAutoInspectTriggerSourceManual})
}

func (s *AnthropicAutoInspectService) ListLogs(ctx context.Context, params pagination.PaginationParams, filter AnthropicAutoInspectLogFilter) ([]AnthropicAutoInspectLog, *pagination.PaginationResult, error) {
	return s.repo.ListLogs(ctx, params, filter)
}

func (s *AnthropicAutoInspectService) ListBatches(ctx context.Context, params pagination.PaginationParams) ([]AnthropicAutoInspectBatch, *pagination.PaginationResult, error) {
	return s.repo.ListBatches(ctx, params)
}

func shouldSkipAnthropicAutoInspect(account Account, now time.Time) bool {
	return !account.Schedulable && account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil)
}

func buildSkippedAnthropicAutoInspectLog(batchID int64, account Account, now time.Time) AnthropicAutoInspectLog {
	return AnthropicAutoInspectLog{
		BatchID:                batchID,
		AccountID:              account.ID,
		AccountNameSnapshot:    account.Name,
		Platform:               account.Platform,
		AccountType:            account.Type,
		Result:                 AnthropicAutoInspectResultSkipped,
		SkipReason:             "already_temp_unschedulable",
		TempUnschedulableUntil: account.TempUnschedulableUntil,
		StartedAt:              now,
		FinishedAt:             now,
	}
}

func classifyAnthropicAutoInspect(result *ScheduledTestResult) AnthropicInspectClassification {
	text := strings.TrimSpace(result.ResponseText + "\n" + result.ErrorMessage)
	lowerText := strings.ToLower(text)
	if strings.EqualFold(strings.TrimSpace(result.Status), "success") {
		return AnthropicInspectClassification{
			Result: AnthropicAutoInspectResultSuccess,
			Reason: truncateAnthropicAutoInspectReason(text),
		}
	}
	if resetAt, ok := parseAnthropicRateLimitResetAt(text); ok {
		return AnthropicInspectClassification{
			Result:  AnthropicAutoInspectResultRateLimited,
			ResetAt: &resetAt,
			Reason:  truncateAnthropicAutoInspectReason(text),
		}
	}
	if strings.Contains(lowerText, "rate limit") || strings.Contains(lowerText, "too many requests") {
		return AnthropicInspectClassification{
			Result: AnthropicAutoInspectResultRateLimited,
			Reason: truncateAnthropicAutoInspectReason(text),
		}
	}
	return AnthropicInspectClassification{
		Result: AnthropicAutoInspectResultError,
		Reason: truncateAnthropicAutoInspectReason(text),
	}
}

func parseAnthropicRateLimitResetAt(text string) (time.Time, bool) {
	for _, pattern := range anthropicAutoInspectResetAtPatterns {
		matched := pattern.FindString(text)
		if matched == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, matched); err == nil {
			return parsed.UTC(), true
		}
		if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", matched, time.UTC); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func truncateAnthropicAutoInspectReason(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "anthropic auto inspect failed"
	}
	if len(trimmed) > 255 {
		return trimmed[:255]
	}
	return trimmed
}
