package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIRoutingExplainRanksCandidatesAndExplainsScore(t *testing.T) {
	cheap := 0.05
	expensive := 0.20
	groupID := int64(9001)
	accounts := []Account{
		{ID: 1, Name: "cheap-fast", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Priority: 1, Concurrency: 5, ChannelPrice: &cheap, GroupIDs: []int64{groupID}},
		{ID: 2, Name: "expensive-fast", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Priority: 1, Concurrency: 5, ChannelPrice: &expensive, GroupIDs: []int64{groupID}},
	}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: accounts},
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
	}
	scheduler := svc.getOpenAIAccountScheduler(context.Background()).(*defaultOpenAIAccountScheduler)
	scheduler.ReportResult(1, true, intPtrForTest(420))
	scheduler.ReportResult(2, true, intPtrForTest(430))

	got, err := svc.ExplainOpenAIRouting(context.Background(), OpenAIRoutingExplainParams{
		GroupID: &groupID,
		Model:   "gpt-5.1",
	})

	require.NoError(t, err)
	require.Len(t, got.Items, 2)
	require.Equal(t, int64(1), got.Items[0].AccountID)
	require.Equal(t, 1, got.Items[0].Rank)
	require.True(t, got.Items[0].IsSchedulableNow)
	require.Greater(t, got.Items[0].Score.Total, got.Items[1].Score.Total)
	require.Greater(t, got.Items[0].Score.Price, got.Items[1].Score.Price)
	require.Equal(t, "成本优", got.Items[0].SummaryReasons[0])
}

func TestOpenAIRoutingExplainReportsPersistentBlockReasons(t *testing.T) {
	resetAt := time.Now().Add(3 * time.Minute)
	groupID := int64(9002)
	accounts := []Account{
		{ID: 3, Name: "blocked-429", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Priority: 1, Concurrency: 5, RateLimitResetAt: &resetAt, GroupIDs: []int64{groupID}},
	}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: accounts},
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
	}

	got, err := svc.ExplainOpenAIRouting(context.Background(), OpenAIRoutingExplainParams{GroupID: &groupID})

	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.False(t, got.Items[0].IsSchedulableNow)
	require.Contains(t, got.Items[0].BlockReasons, OpenAIRoutingReasonRateLimited)
	require.Equal(t, "跳过", got.Items[0].StatusLabel)
}
