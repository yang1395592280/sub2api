package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type businessAnalyticsRepoStub struct {
	overview BusinessOverviewData
	trend    []BusinessTrendPoint
	groups   []BusinessGroupRow
}

func (r *businessAnalyticsRepoStub) GetOverview(context.Context, BusinessAnalyticsFilter) (*BusinessOverviewData, error) {
	return &r.overview, nil
}

func (r *businessAnalyticsRepoStub) GetTrend(context.Context, BusinessAnalyticsFilter) ([]BusinessTrendPoint, error) {
	return r.trend, nil
}

func (r *businessAnalyticsRepoStub) GetGroups(context.Context, BusinessAnalyticsFilter) ([]BusinessGroupRow, error) {
	return r.groups, nil
}

func (r *businessAnalyticsRepoStub) GetChannels(context.Context, BusinessAnalyticsFilter) ([]BusinessChannelRow, error) {
	return nil, nil
}

func (r *businessAnalyticsRepoStub) GetPriceChangeImpact(context.Context, PriceChangeImpactInput) (*PriceChangeImpactResponse, error) {
	return nil, nil
}

func (r *businessAnalyticsRepoStub) GetRecords(context.Context, BusinessRecordsFilter) (*BusinessRecordsResponse, error) {
	return nil, nil
}

func TestProfitMargin(t *testing.T) {
	require.Nil(t, ProfitMargin(0, 1))
	require.InDelta(t, 0.4, *ProfitMargin(10, 4), 0.000001)
}

func TestBusinessAnalyticsService_GetOverviewDerivedMetrics(t *testing.T) {
	svc := NewBusinessAnalyticsService(&businessAnalyticsRepoStub{
		overview: BusinessOverviewData{
			ActiveUsers: 2,
			Revenue:     10,
			GrossProfit: 4,
		},
		trend: []BusinessTrendPoint{{Date: "2026-06-01", Revenue: 5, GrossProfit: 1}},
	})
	filter := BusinessAnalyticsFilter{
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
	}

	got, err := svc.GetOverview(context.Background(), filter)

	require.NoError(t, err)
	require.InDelta(t, 0.4, *got.ProfitMargin, 0.000001)
	require.InDelta(t, 5, *got.RevenuePerActiveUser, 0.000001)
	require.InDelta(t, 2, *got.ProfitPerActiveUser, 0.000001)
	require.InDelta(t, 0.2, *got.Trend[0].ProfitMargin, 0.000001)
}

func TestBusinessAnalyticsService_GetGroupsAddsComparisonMetrics(t *testing.T) {
	svc := NewBusinessAnalyticsService(&businessAnalyticsRepoStub{
		groups: []BusinessGroupRow{{
			Revenue:             12,
			GrossProfit:         6,
			PreviousRevenue:     8,
			PreviousGrossProfit: 3,
		}},
	})

	rows, err := svc.GetGroups(context.Background(), BusinessAnalyticsFilter{})

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.InDelta(t, 0.5, *rows[0].ProfitMargin, 0.000001)
	require.InDelta(t, 0.5, *rows[0].RevenueChangeRate, 0.000001)
	require.InDelta(t, 1, *rows[0].GrossProfitChangeRate, 0.000001)
}
