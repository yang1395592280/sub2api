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
}

func (s *anthropicAutoInspectAccountRepoStub) ListByPlatform(_ context.Context, _ string) ([]Account, error) {
	return append([]Account(nil), s.accounts...), nil
}

func (s *anthropicAutoInspectAccountRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
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
}

func (s *anthropicAutoInspectRepoStub) CreateBatch(_ context.Context, _ CreateAnthropicAutoInspectBatchInput) (int64, error) {
	s.nextBatchID++
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
	s.logs = append(s.logs, log)
	return nil
}

func (s *anthropicAutoInspectRepoStub) ListLogs(context.Context, pagination.PaginationParams, AnthropicAutoInspectLogFilter) ([]AnthropicAutoInspectLog, *pagination.PaginationResult, error) {
	panic("unexpected ListLogs call")
}

func (s *anthropicAutoInspectRepoStub) ListBatches(context.Context, pagination.PaginationParams) ([]AnthropicAutoInspectBatch, *pagination.PaginationResult, error) {
	panic("unexpected ListBatches call")
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

	err := svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{TriggerSource: AnthropicAutoInspectTriggerSourceScheduler})
	require.NoError(t, err)
	require.WithinDuration(t, now.Add(30*time.Minute), repo.lastTempUnschedUntil[2], time.Second)
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
	require.NoError(t, svc.RunBatch(context.Background(), AnthropicAutoInspectRunInput{TriggerSource: AnthropicAutoInspectTriggerSourceScheduler}))
	require.Equal(t, []int64{11, 22}, callOrder)
}

func anthropicPtrTime(value time.Time) *time.Time {
	return &value
}
