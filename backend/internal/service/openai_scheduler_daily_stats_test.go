package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type openAISchedulerStatsRepoStub struct {
	increments []openAISchedulerStatsIncrementCall
}

type openAISchedulerStatsIncrementCall struct {
	Date       time.Time
	GroupID    int64
	AccountID  int64
	SelectedAt time.Time
}

func (r *openAISchedulerStatsRepoStub) IncrementDailySelection(ctx context.Context, statDate time.Time, groupID int64, accountID int64, selectedAt time.Time) error {
	_ = ctx
	r.increments = append(r.increments, openAISchedulerStatsIncrementCall{
		Date:       statDate,
		GroupID:    groupID,
		AccountID:  accountID,
		SelectedAt: selectedAt,
	})
	return nil
}

func (r *openAISchedulerStatsRepoStub) GetDailyStats(ctx context.Context, statDate time.Time, groupID int64) (*OpenAISchedulerDailyStats, error) {
	_ = ctx
	return &OpenAISchedulerDailyStats{Date: statDate.Format("2006-01-02"), GroupID: groupID}, nil
}

func (r *openAISchedulerStatsRepoStub) RecomputeDailyStatsFromUsageLogs(ctx context.Context, statDate time.Time, start time.Time, end time.Time, groupID int64) (*OpenAISchedulerDailyStats, error) {
	_ = ctx
	_ = start
	_ = end
	return &OpenAISchedulerDailyStats{Date: statDate.Format("2006-01-02"), GroupID: groupID}, nil
}

func TestOpenAIGatewayService_RecordOpenAISchedulerDailySelection_UsesSelectedGroup(t *testing.T) {
	groupID := int64(33)
	selectedAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	repo := &openAISchedulerStatsRepoStub{}
	svc := &OpenAIGatewayService{openAISchedulerStatsRepo: repo}

	svc.recordOpenAISchedulerDailySelection(context.Background(), &groupID, 11855, selectedAt)

	require.Len(t, repo.increments, 1)
	require.Equal(t, groupID, repo.increments[0].GroupID)
	require.Equal(t, int64(11855), repo.increments[0].AccountID)
	require.Equal(t, selectedAt, repo.increments[0].SelectedAt)
	require.Equal(t, 2026, repo.increments[0].Date.Year())
	require.Equal(t, time.June, repo.increments[0].Date.Month())
	require.Equal(t, 13, repo.increments[0].Date.Day())
}

func TestOpenAIGatewayService_RecordOpenAISchedulerDailySelection_SkipsInvalidScope(t *testing.T) {
	groupID := int64(33)
	repo := &openAISchedulerStatsRepoStub{}
	svc := &OpenAIGatewayService{openAISchedulerStatsRepo: repo}

	svc.recordOpenAISchedulerDailySelection(context.Background(), nil, 11855, time.Now())
	svc.recordOpenAISchedulerDailySelection(context.Background(), &groupID, 0, time.Now())

	require.Empty(t, repo.increments)
}

func TestDefaultOpenAIAccountScheduler_SelectRecordsDailySelectionForGroup(t *testing.T) {
	groupID := int64(44)
	account := Account{
		ID:          11855,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		GroupIDs:    []int64{groupID},
	}
	statsRepo := &openAISchedulerStatsRepoStub{}
	svc := &OpenAIGatewayService{
		accountRepo:              schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{account}}},
		cache:                    &schedulerTestGatewayCache{},
		cfg:                      &config.Config{},
		concurrencyService:       NewConcurrencyService(schedulerTestConcurrencyCache{}),
		openAISchedulerStatsRepo: statsRepo,
		openaiAccountStats:       newOpenAIAccountRuntimeStats(),
	}
	scheduler := &defaultOpenAIAccountScheduler{
		service:        svc,
		stats:          newOpenAIAccountRuntimeStats(),
		healthSettings: defaultOpenAISchedulerHealthSettings(),
	}

	selection, decision, err := scheduler.Select(context.Background(), OpenAIAccountScheduleRequest{
		GroupID:     &groupID,
		SessionHash: "daily-stats",
	})

	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, account.ID, decision.SelectedAccountID)
	require.Len(t, statsRepo.increments, 1)
	require.Equal(t, groupID, statsRepo.increments[0].GroupID)
	require.Equal(t, account.ID, statsRepo.increments[0].AccountID)
}
