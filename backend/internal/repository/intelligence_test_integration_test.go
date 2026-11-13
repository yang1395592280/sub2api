//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIntelligenceIntegrationKindsCostsSchedulesAndConcurrency(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	g := mustCreateGroup(t, client, &service.Group{Name: uuid.NewString(), Platform: service.PlatformOpenAI})
	a := mustCreateAccount(t, client, &service.Account{Name: uuid.NewString(), Platform: service.PlatformOpenAI})
	_, err := integrationDB.Exec(`INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, a.ID, g.ID)
	require.NoError(t, err)
	r := NewPelicanTestRepository(integrationDB)
	pelican, err := r.GetGroupConfig(ctx, g.ID)
	require.NoError(t, err)
	require.False(t, pelican.Enabled)
	intelligence, err := r.GetGroupConfig(ctx, g.ID, service.TestKindIntelligence)
	require.NoError(t, err)
	require.False(t, intelligence.Enabled)
	require.Equal(t, service.DefaultIntelligenceTestPrompt, intelligence.Prompt)
	p, err := r.CreateBatch(ctx, uuid.NewString(), []int64{a.ID}, "gpt-5.5", service.DefaultPelicanTestPrompt)
	require.NoError(t, err)
	require.Len(t, p, 1)
	i, err := r.CreateBatch(ctx, uuid.NewString(), []int64{a.ID}, "gpt-5.5", service.DefaultIntelligenceTestPrompt, service.TestKindIntelligence)
	require.NoError(t, err)
	require.Len(t, i, 1)
	duplicate, err := r.CreateBatch(ctx, uuid.NewString(), []int64{a.ID}, "gpt-5.5", service.DefaultIntelligenceTestPrompt, service.TestKindIntelligence)
	require.NoError(t, err)
	require.Empty(t, duplicate)
	_, err = r.Get(ctx, i[0].ID)
	require.ErrorIs(t, err, sql.ErrNoRows, "pelican detail endpoint cannot expose an intelligence record")
	latest, err := r.ListLatest(ctx, []int64{a.ID})
	require.NoError(t, err)
	require.Equal(t, p[0].ID, latest[0].ID)
	latest, err = r.ListLatest(ctx, []int64{a.ID}, service.TestKindIntelligence)
	require.NoError(t, err)
	require.Equal(t, i[0].ID, latest[0].ID)
	cost, rate, count := .0000628, .1, int64(150)
	require.NoError(t, r.Finish(ctx, i[0].ID, "correct", "20", "", "", 250, service.AccountTestTokenUsage{TotalTokens: &count}, service.PelicanTest{Answer: "20", EstimatedCostUSD: &cost, UpstreamRateMultiplier: &rate, CostModelID: "gpt-5.5"}))
	require.NoError(t, r.Finish(ctx, i[0].ID, "failed", "", "", "second finish", 1, service.AccountTestTokenUsage{}))
	detail, err := r.Get(ctx, i[0].ID, service.TestKindIntelligence)
	require.NoError(t, err)
	require.Equal(t, "20", detail.Answer)
	require.InDelta(t, cost, *detail.EstimatedCostUSD, 1e-12)
	history, err := r.ListHistory(ctx, a.ID, 1, 20, service.TestKindIntelligence)
	require.NoError(t, err)
	require.EqualValues(t, 1, history.Total)
	require.Empty(t, history.Items[0].ResponseText)
	require.Equal(t, "20", history.Items[0].Answer)
	require.NoError(t, r.Finish(ctx, p[0].ID, "previewable", "html", "<html></html>", "", 300, service.AccountTestTokenUsage{}))
	intelligence.Enabled, intelligence.ModelID = true, "gpt-5.5"
	require.NoError(t, r.SaveGroupConfig(ctx, *intelligence))
	require.NoError(t, r.SaveGroupConfig(ctx, *pelican))
	require.NoError(t, r.EnqueueDueGroups(ctx))
	latest, err = r.ListLatest(ctx, []int64{a.ID}, service.TestKindIntelligence)
	require.NoError(t, err)
	require.Len(t, latest, 1)
	require.Equal(t, "queued", latest[0].Status)
	require.Equal(t, g.Name, latest[0].SourceGroupName)
	queued, err := r.Get(ctx, latest[0].ID, service.TestKindIntelligence)
	require.NoError(t, err)
	enabled, err := r.ScheduledTestEnabled(ctx, *queued)
	require.NoError(t, err)
	require.True(t, enabled)
	intelligence.Enabled = false
	require.NoError(t, r.SaveGroupConfig(ctx, *intelligence))
	enabled, err = r.ScheduledTestEnabled(ctx, *queued)
	require.NoError(t, err)
	require.False(t, enabled)
	// Both kinds use the same global count/claim lock and three execution slots.
	ids := []int64{a.ID}
	for n := 0; n < 3; n++ {
		account := mustCreateAccount(t, client, &service.Account{Name: uuid.NewString(), Platform: service.PlatformOpenAI})
		ids = append(ids, account.ID)
		_, err = r.CreateBatch(ctx, uuid.NewString(), []int64{account.ID}, "gpt-5.5", service.DefaultPelicanTestPrompt)
		require.NoError(t, err)
	}
	for n := 0; n < 3; n++ {
		claimed, err := r.ClaimNext(ctx)
		require.NoError(t, err)
		require.NotNil(t, claimed)
	}
	claimed, err := r.ClaimNext(ctx)
	require.NoError(t, err)
	require.Nil(t, claimed)
	for _, id := range ids {
		_, err = integrationDB.Exec(`UPDATE account_pelican_tests SET status='failed',finished_at=NOW() WHERE account_id=$1 AND status IN ('queued','running')`, id)
		require.NoError(t, err)
	}
}
