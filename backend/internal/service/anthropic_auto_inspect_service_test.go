//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type anthropicAutoInspectAccountRepoStub struct {
	accounts              []Account
	lastTempUnschedUntil  map[int64]time.Time
	lastTempUnschedReason map[int64]string
	setTempUnschedErr     error
}

func (s *anthropicAutoInspectAccountRepoStub) ListByPlatform(_ context.Context, _ string) ([]Account, error) {
	return append([]Account(nil), s.accounts...), nil
}

func (s *anthropicAutoInspectAccountRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	if s.setTempUnschedErr != nil {
		return s.setTempUnschedErr
	}
	if s.lastTempUnschedUntil == nil {
		s.lastTempUnschedUntil = make(map[int64]time.Time)
	}
	if s.lastTempUnschedReason == nil {
		s.lastTempUnschedReason = make(map[int64]string)
	}
	s.lastTempUnschedUntil[id] = until
	s.lastTempUnschedReason[id] = reason
	return nil
}

type anthropicAutoInspectTestRunnerStub struct {
	results map[int64]*ScheduledTestResult
	runFn   func(ctx context.Context, accountID int64, modelID string) (*ScheduledTestResult, error)
	calls   []int64
}

func (s *anthropicAutoInspectTestRunnerStub) RunTestBackground(ctx context.Context, accountID int64, modelID string) (*ScheduledTestResult, error) {
	s.calls = append(s.calls, accountID)
	if s.runFn != nil {
		return s.runFn(ctx, accountID, modelID)
	}
	if result, ok := s.results[accountID]; ok {
		return result, nil
	}
	return &ScheduledTestResult{Status: "success", StartedAt: time.Now(), FinishedAt: time.Now()}, nil
}

type anthropicAutoInspectRepoStub struct {
	nextBatchID int64
	logs        []AnthropicAutoInspectLog
	completed   []AnthropicAutoInspectBatchStats
	failed      []AnthropicAutoInspectBatchStats
	skipped     []CreateAnthropicAutoInspectSkippedBatchInput
	createLogErr error
}

func (s *anthropicAutoInspectRepoStub) CreateBatch(_ context.Context, _ CreateAnthropicAutoInspectBatchInput) (int64, error) {
	s.nextBatchID++
	return s.nextBatchID, nil
}

func (s *anthropicAutoInspectRepoStub) CreateSkippedBatch(_ context.Context, input CreateAnthropicAutoInspectSkippedBatchInput) (int64, error) {
	s.nextBatchID++
	s.skipped = append(s.skipped, input)
	return s.nextBatchID, nil
}

func (s *anthropicAutoInspectRepoStub) CompleteBatch(_ context.Context, _ int64, stats AnthropicAutoInspectBatchStats, _ time.Time) error {
	s.completed = append(s.completed, stats)
	return nil
}

func (s *anthropicAutoInspectRepoStub) MarkBatchFailed(_ context.Context, _ int64, stats AnthropicAutoInspectBatchStats, _ time.Time) error {
	s.failed = append(s.failed, stats)
	return nil
}

func (s *anthropicAutoInspectRepoStub) CreateLog(_ context.Context, log AnthropicAutoInspectLog) error {
	if s.createLogErr != nil {
		return s.createLogErr
	}
	s.logs = append(s.logs, log)
	return nil
}

func (s *anthropicAutoInspectRepoStub) ListLogs(context.Context, pagination.PaginationParams, AnthropicAutoInspectLogFilter) ([]AnthropicAutoInspectLog, *pagination.PaginationResult, error) {
	panic("unexpected ListLogs call")
}

func (s *anthropicAutoInspectRepoStub) ListBatches(context.Context, pagination.PaginationParams) ([]AnthropicAutoInspectBatch, *pagination.PaginationResult, error) {
	panic("unexpected ListBatches call")
}

type anthropicAutoInspectSettingsProviderStub struct {
	settings *SystemSettings
}

func (s *anthropicAutoInspectSettingsProviderStub) GetAllSettings(context.Context) (*SystemSettings, error) {
	if s.settings == nil {
		return &SystemSettings{}, nil
	}
	copy := *s.settings
	return &copy, nil
}

func (s *anthropicAutoInspectSettingsProviderStub) UpdateSettings(context.Context, *SystemSettings) error {
	return nil
}

func TestAnthropicAutoInspectService_RunBatch_SkipsAlreadyTempUnschedulable(t *testing.T) {
	t.Parallel()

	repo := &anthropicAutoInspectAccountRepoStub{
		accounts: []Account{
			{
				ID:                     1,
				Name:                   "anthropic-1",
				Platform:               PlatformAnthropic,
				Type:                   AccountTypeAPIKey,
				Schedulable:            false,
				TempUnschedulableUntil: anthropicPtrTime(time.Now().Add(5 * time.Minute)),
			},
		},
	}
	testRunner := &anthropicAutoInspectTestRunnerStub{}
	logRepo := &anthropicAutoInspectRepoStub{}

	svc := NewAnthropicAutoInspectService(repo, testRunner, logRepo, &config.Config{})
	err := svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{TriggerSource: AnthropicAutoInspectTriggerSourceManual})
	require.NoError(t, err)
	require.Empty(t, testRunner.calls)
	require.Len(t, logRepo.logs, 1)
	require.Equal(t, AnthropicAutoInspectResultSkipped, logRepo.logs[0].Result)
}

func TestAnthropicAutoInspectService_RunBatch_SkipsWhenDisabledForScheduler(t *testing.T) {
	t.Parallel()

	repo := &anthropicAutoInspectAccountRepoStub{
		accounts: []Account{
			{ID: 1, Name: "anthropic-1", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Schedulable: true},
		},
	}
	testRunner := &anthropicAutoInspectTestRunnerStub{}
	logRepo := &anthropicAutoInspectRepoStub{}
	settingsProvider := &anthropicAutoInspectSettingsProviderStub{
		settings: &SystemSettings{
			AnthropicAutoInspectEnabled:              false,
			AnthropicAutoInspectIntervalMinutes:      1,
			AnthropicAutoInspectErrorCooldownMinutes: 30,
		},
	}

	svc := NewAnthropicAutoInspectService(repo, testRunner, logRepo, &config.Config{})
	svc.SetSettingsProvider(settingsProvider)

	err := svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{
		TriggerSource: AnthropicAutoInspectTriggerSourceScheduler,
	})
	require.NoError(t, err)
	require.Empty(t, testRunner.calls)
	require.Empty(t, logRepo.logs)
	require.Zero(t, logRepo.nextBatchID)
}

func TestAnthropicAutoInspectService_RunBatch_UsesThirtyMinuteCooldownForGenericError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	repo := &anthropicAutoInspectAccountRepoStub{
		accounts: []Account{{ID: 2, Name: "anthropic-2", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Schedulable: true}},
	}
	testRunner := &anthropicAutoInspectTestRunnerStub{
		results: map[int64]*ScheduledTestResult{
			2: {
				Status:       "failed",
				ErrorMessage: "upstream 500 temporary failure",
				StartedAt:    now,
				FinishedAt:   now.Add(2 * time.Second),
			},
		},
	}

	svc := NewAnthropicAutoInspectService(repo, testRunner, &anthropicAutoInspectRepoStub{}, &config.Config{})
	svc.now = func() time.Time { return now }
	svc.SetSettingsProvider(&anthropicAutoInspectSettingsProviderStub{
		settings: &SystemSettings{
			AnthropicAutoInspectEnabled:              true,
			AnthropicAutoInspectIntervalMinutes:      1,
			AnthropicAutoInspectErrorCooldownMinutes: 30,
		},
	})

	err := svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{TriggerSource: AnthropicAutoInspectTriggerSourceScheduler})
	require.NoError(t, err)
	require.WithinDuration(t, now.Add(30*time.Minute), repo.lastTempUnschedUntil[2], time.Second)
}

func TestAnthropicAutoInspectService_RunBatch_UsesPerAccountTimeout(t *testing.T) {
	t.Parallel()

	repo := &anthropicAutoInspectAccountRepoStub{
		accounts: []Account{
			{ID: 1, Name: "slow", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Schedulable: true},
			{ID: 2, Name: "next", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Schedulable: true},
		},
	}

	testRunner := &anthropicAutoInspectTestRunnerStub{
		runFn: func(ctx context.Context, accountID int64, _ string) (*ScheduledTestResult, error) {
			if accountID == 1 {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			return &ScheduledTestResult{
				Status:     "success",
				StartedAt:  time.Now(),
				FinishedAt: time.Now(),
			}, nil
		},
	}
	logRepo := &anthropicAutoInspectRepoStub{}

	svc := NewAnthropicAutoInspectService(repo, testRunner, logRepo, &config.Config{})
	svc.SetSettingsProvider(&anthropicAutoInspectSettingsProviderStub{
		settings: &SystemSettings{
			AnthropicAutoInspectEnabled:              true,
			AnthropicAutoInspectIntervalMinutes:      1,
			AnthropicAutoInspectErrorCooldownMinutes: 30,
		},
	})
	svc.perAccountTimeout = 5 * time.Millisecond

	err := svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{
		TriggerSource: AnthropicAutoInspectTriggerSourceScheduler,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, testRunner.calls)
	require.Len(t, logRepo.logs, 2)
	require.Equal(t, AnthropicAutoInspectResultError, logRepo.logs[0].Result)
	require.Equal(t, AnthropicAutoInspectResultSuccess, logRepo.logs[1].Result)
}

func TestAnthropicAutoInspectService_RunBatch_ProcessesAccountsSerially(t *testing.T) {
	t.Parallel()

	callOrder := make([]int64, 0, 2)
	testRunner := &anthropicAutoInspectTestRunnerStub{
		runFn: func(_ context.Context, accountID int64, _ string) (*ScheduledTestResult, error) {
			callOrder = append(callOrder, accountID)
			return &ScheduledTestResult{
				Status:     "success",
				StartedAt:  time.Now(),
				FinishedAt: time.Now(),
			}, nil
		},
	}

	repo := &anthropicAutoInspectAccountRepoStub{
		accounts: []Account{
			{ID: 22, Name: "anthropic-22", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Schedulable: true},
			{ID: 11, Name: "anthropic-11", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Schedulable: true},
		},
	}

	svc := NewAnthropicAutoInspectService(repo, testRunner, &anthropicAutoInspectRepoStub{}, &config.Config{})
	svc.SetSettingsProvider(&anthropicAutoInspectSettingsProviderStub{
		settings: &SystemSettings{
			AnthropicAutoInspectEnabled:              true,
			AnthropicAutoInspectIntervalMinutes:      1,
			AnthropicAutoInspectErrorCooldownMinutes: 30,
		},
	})
	require.NoError(t, svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{TriggerSource: AnthropicAutoInspectTriggerSourceScheduler}))
	require.Equal(t, []int64{11, 22}, callOrder)
}

func TestParseAnthropicRateLimitResetAt_AcceptsSpaceSeparatedTimestamp(t *testing.T) {
	t.Parallel()

	resetAt, ok := parseAnthropicRateLimitResetAt("rate limited until 2026-04-26 12:34:56")
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 4, 26, 12, 34, 56, 0, time.UTC), resetAt)
}

func TestAnthropicAutoInspectService_RunBatch_PersistsSkippedBatchWhenAlreadyRunning(t *testing.T) {
	t.Parallel()

	logRepo := &anthropicAutoInspectRepoStub{}
	svc := NewAnthropicAutoInspectService(
		&anthropicAutoInspectAccountRepoStub{},
		&anthropicAutoInspectTestRunnerStub{},
		logRepo,
		&config.Config{},
	)
	svc.SetSettingsProvider(&anthropicAutoInspectSettingsProviderStub{
		settings: &SystemSettings{
			AnthropicAutoInspectEnabled:              true,
			AnthropicAutoInspectIntervalMinutes:      1,
			AnthropicAutoInspectErrorCooldownMinutes: 30,
		},
	})
	svc.running.Store(true)

	err := svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{
		TriggerSource: AnthropicAutoInspectTriggerSourceScheduler,
	})
	require.NoError(t, err)
	require.Len(t, logRepo.skipped, 1)
	require.Equal(t, "batch_already_running", logRepo.skipped[0].SkipReason)
	require.Equal(t, AnthropicAutoInspectTriggerSourceScheduler, logRepo.skipped[0].TriggerSource)
}

func TestAnthropicAutoInspectService_RunBatch_FailsWhenTempUnschedulableWriteFails(t *testing.T) {
	t.Parallel()

	repo := &anthropicAutoInspectAccountRepoStub{
		accounts: []Account{
			{ID: 7, Name: "anthropic-7", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Schedulable: true},
		},
		setTempUnschedErr: context.DeadlineExceeded,
	}
	testRunner := &anthropicAutoInspectTestRunnerStub{
		results: map[int64]*ScheduledTestResult{
			7: {
				Status:       "failed",
				ErrorMessage: "rate limited until 2026-04-26T12:34:56Z",
				StartedAt:    time.Date(2026, 4, 26, 12, 30, 0, 0, time.UTC),
				FinishedAt:   time.Date(2026, 4, 26, 12, 30, 1, 0, time.UTC),
			},
		},
	}
	logRepo := &anthropicAutoInspectRepoStub{}

	svc := NewAnthropicAutoInspectService(repo, testRunner, logRepo, &config.Config{})
	svc.SetSettingsProvider(&anthropicAutoInspectSettingsProviderStub{
		settings: &SystemSettings{
			AnthropicAutoInspectEnabled:              true,
			AnthropicAutoInspectIntervalMinutes:      1,
			AnthropicAutoInspectErrorCooldownMinutes: 30,
		},
	})

	err := svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{
		TriggerSource: AnthropicAutoInspectTriggerSourceScheduler,
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Empty(t, logRepo.completed)
	require.Len(t, logRepo.failed, 1)
	require.Empty(t, logRepo.logs)
}

func TestAnthropicAutoInspectService_RunBatch_FailsWhenSkippedLogPersistFails(t *testing.T) {
	t.Parallel()

	logRepo := &anthropicAutoInspectRepoStub{createLogErr: context.Canceled}
	repo := &anthropicAutoInspectAccountRepoStub{
		accounts: []Account{
			{
				ID:                     1,
				Name:                   "anthropic-1",
				Platform:               PlatformAnthropic,
				Type:                   AccountTypeAPIKey,
				Schedulable:            false,
				TempUnschedulableUntil: anthropicPtrTime(time.Now().Add(5 * time.Minute)),
			},
		},
	}

	svc := NewAnthropicAutoInspectService(repo, &anthropicAutoInspectTestRunnerStub{}, logRepo, &config.Config{})
	err := svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{TriggerSource: AnthropicAutoInspectTriggerSourceManual})
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, logRepo.completed)
	require.Len(t, logRepo.failed, 1)
}

func TestAnthropicAutoInspectService_RunBatch_FailsWhenInspectLogPersistFails(t *testing.T) {
	t.Parallel()

	logRepo := &anthropicAutoInspectRepoStub{createLogErr: context.Canceled}
	repo := &anthropicAutoInspectAccountRepoStub{
		accounts: []Account{
			{ID: 1, Name: "anthropic-1", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Schedulable: true},
		},
	}
	testRunner := &anthropicAutoInspectTestRunnerStub{
		results: map[int64]*ScheduledTestResult{
			1: {
				Status:     "success",
				StartedAt:  time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
				FinishedAt: time.Date(2026, 4, 26, 12, 0, 1, 0, time.UTC),
			},
		},
	}

	svc := NewAnthropicAutoInspectService(repo, testRunner, logRepo, &config.Config{})
	err := svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{TriggerSource: AnthropicAutoInspectTriggerSourceManual})
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, logRepo.completed)
	require.Len(t, logRepo.failed, 1)
}

func anthropicPtrTime(value time.Time) *time.Time {
	return &value
}
