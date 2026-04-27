//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAnthropicAutoInspectRepository_CreateBatchLogAndListLogs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tx := testTx(t)
	repo := &anthropicAutoInspectRepository{db: tx}

	startedAt := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Minute)
	recoverAt := startedAt.Add(10 * time.Minute)

	batchID, err := repo.CreateBatch(ctx, service.CreateAnthropicAutoInspectBatchInput{
		TriggerSource: service.AnthropicAutoInspectTriggerSourceManual,
		Status:        service.AnthropicAutoInspectBatchStatusRunning,
		StartedAt:     startedAt,
	})
	require.NoError(t, err)
	require.Positive(t, batchID)

	var triggerSource string
	var status string
	require.NoError(t, tx.QueryRowContext(ctx, `
		SELECT trigger_source, status
		FROM anthropic_auto_inspect_batches
		WHERE id = $1
	`, batchID).Scan(&triggerSource, &status))
	require.Equal(t, service.AnthropicAutoInspectTriggerSourceManual, triggerSource)
	require.Equal(t, service.AnthropicAutoInspectBatchStatusRunning, status)

	err = repo.CreateLog(ctx, service.AnthropicAutoInspectLog{
		BatchID:                batchID,
		AccountID:              101,
		AccountNameSnapshot:    "demo-account",
		Platform:               service.PlatformAnthropic,
		AccountType:            service.AccountTypeAPIKey,
		Result:                 service.AnthropicAutoInspectResultRateLimited,
		ResponseText:           "rate limited until 2026-04-26T12:10:00Z",
		TempUnschedulableUntil: &recoverAt,
		SchedulableChanged:     true,
		StartedAt:              startedAt,
		FinishedAt:             finishedAt,
		LatencyMs:              finishedAt.Sub(startedAt).Milliseconds(),
	})
	require.NoError(t, err)

	logs, page, err := repo.ListLogs(ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, service.AnthropicAutoInspectLogFilter{})
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Len(t, logs, 1)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, service.AnthropicAutoInspectResultRateLimited, logs[0].Result)
	require.Equal(t, "demo-account", logs[0].AccountNameSnapshot)
	require.NotNil(t, logs[0].TempUnschedulableUntil)
	require.Equal(t, recoverAt, logs[0].TempUnschedulableUntil.UTC())

	filteredLogs, _, err := repo.ListLogs(ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, service.AnthropicAutoInspectLogFilter{
		Search:      "2026-04-26T12:10:00Z",
		StartedFrom: anthropicPtrTime(startedAt.Add(-time.Second)),
		StartedTo:   anthropicPtrTime(startedAt.Add(time.Second)),
	})
	require.NoError(t, err)
	require.Len(t, filteredLogs, 1)
}

func TestAnthropicAutoInspectRepository_CreateSkippedBatchAndListBatches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tx := testTx(t)
	repo := &anthropicAutoInspectRepository{db: tx}

	now := time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC)
	batchID, err := repo.CreateSkippedBatch(ctx, service.CreateAnthropicAutoInspectSkippedBatchInput{
		TriggerSource: service.AnthropicAutoInspectTriggerSourceScheduler,
		SkipReason:    "batch_already_running",
		StartedAt:     now,
		FinishedAt:    now,
	})
	require.NoError(t, err)
	require.Positive(t, batchID)

	batches, page, err := repo.ListBatches(ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.NotNil(t, page)
	require.NotEmpty(t, batches)
	require.Equal(t, service.AnthropicAutoInspectBatchStatusSkipped, batches[0].Status)
	require.Equal(t, "batch_already_running", batches[0].SkipReason)
}

func anthropicPtrTime(value time.Time) *time.Time {
	return &value
}
