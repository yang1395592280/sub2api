package service

import (
	"context"
	"errors"
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
	require.Equal(t, "candidate", got.Items[0].StatusLabel)
	require.Equal(t, "cost_advantage", got.Items[0].SummaryReasons[0])
}

func TestOpenAIRoutingExplainReportsStructuredCooldownDetails(t *testing.T) {
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
	require.Equal(t, "skipped", got.Items[0].StatusLabel)
	require.Len(t, got.Items[0].BlockDetails, 1)
	require.Equal(t, OpenAIRoutingReasonRateLimited, got.Items[0].BlockDetails[0].Reason)
	require.Equal(t, "ui_countdown_state", got.Items[0].BlockDetails[0].Source)
	require.NotNil(t, got.Items[0].BlockDetails[0].Until)
	require.WithinDuration(t, resetAt, *got.Items[0].BlockDetails[0].Until, time.Second)
	require.WithinDuration(t, got.Items[0].SnapshotAt, got.Items[0].BlockDetails[0].SnapshotAt, time.Second)
}

func TestOpenAIRoutingExplainForAccountLoadsManualUnschedulableAccountByID(t *testing.T) {
	groupID := int64(9003)
	accounts := []Account{
		{ID: 11, Name: "candidate", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Priority: 1, Concurrency: 5, GroupIDs: []int64{groupID}},
		{ID: 12, Name: "manual-off", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: false, Priority: 1, Concurrency: 5, GroupIDs: []int64{groupID}},
	}
	svc := &OpenAIGatewayService{
		accountRepo: schedulerExplainRankingRepo{
			rankingAccounts: []Account{accounts[0]},
			accountsByID: map[int64]Account{
				accounts[0].ID: accounts[0],
				accounts[1].ID: accounts[1],
			},
		},
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
	}

	got, err := svc.ExplainOpenAIRoutingForAccount(context.Background(), 12, OpenAIRoutingExplainParams{GroupID: &groupID})

	require.NoError(t, err)
	require.Equal(t, int64(12), got.Account.AccountID)
	require.False(t, got.Account.IsSchedulableNow)
	require.Equal(t, "skipped", got.Account.StatusLabel)
	require.Contains(t, got.Account.BlockReasons, OpenAIRoutingReasonManualUnschedulable)
	require.Len(t, got.Top, 1)
	require.Equal(t, int64(11), got.Top[0].AccountID)
	require.Equal(t, []string{"sticky_may_override_ranking", "weighted_top_k_not_strict_best"}, got.Notes)
}

func TestOpenAIRoutingExplainForAccountRejectsPlatformMismatch(t *testing.T) {
	groupID := int64(9004)
	anthropic := Account{ID: 21, Name: "other-platform", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: false}
	svc := &OpenAIGatewayService{
		accountRepo: schedulerExplainRankingRepo{
			rankingAccounts: nil,
			accountsByID: map[int64]Account{
				anthropic.ID: anthropic,
			},
		},
	}

	_, err := svc.ExplainOpenAIRoutingForAccount(context.Background(), anthropic.ID, OpenAIRoutingExplainParams{GroupID: &groupID})

	require.ErrorIs(t, err, ErrAccountNotFound)
}

type schedulerExplainRankingRepo struct {
	schedulerTestOpenAIAccountRepo
	rankingAccounts []Account
	accountsByID    map[int64]Account
}

func (r schedulerExplainRankingRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	account, ok := r.accountsByID[id]
	if !ok {
		return nil, ErrAccountNotFound
	}
	copy := account
	return &copy, nil
}

func (r schedulerExplainRankingRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]Account, error) {
	return r.listRanking(platform), nil
}

func (r schedulerExplainRankingRepo) ListSchedulableByPlatform(_ context.Context, platform string) ([]Account, error) {
	return r.listRanking(platform), nil
}

func (r schedulerExplainRankingRepo) ListSchedulableUngroupedByPlatform(_ context.Context, platform string) ([]Account, error) {
	return r.listRanking(platform), nil
}

func (r schedulerExplainRankingRepo) listRanking(platform string) []Account {
	result := make([]Account, 0, len(r.rankingAccounts))
	for _, account := range r.rankingAccounts {
		if account.Platform == platform {
			result = append(result, account)
		}
	}
	return result
}

func TestSchedulerExplainRankingRepoMissingAccountReturnsNotFound(t *testing.T) {
	repo := schedulerExplainRankingRepo{}

	got, err := repo.GetByID(context.Background(), 404)

	require.Nil(t, got)
	require.True(t, errors.Is(err, ErrAccountNotFound))
}
