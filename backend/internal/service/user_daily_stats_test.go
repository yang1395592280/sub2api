//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeUserDailyStatsQuery(t *testing.T) {
	now := time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)
	q, err := normalizeUserDailyStatsQuery(UserDailyStatsFilter{PageSize: 500, Search: "  用户  "}, now)
	require.NoError(t, err)
	require.Equal(t, "2026-11-13", q.Date)
	require.Equal(t, now, q.Start.UTC())
	require.Equal(t, 24*time.Hour, q.End.Sub(q.Start))
	require.Equal(t, "2026-11-12", q.PreviousStart.Format(time.DateOnly))
	require.Equal(t, 100, q.PageSize)
	require.Equal(t, 1, q.Page)
	require.Equal(t, "用户", q.Search)
	require.Equal(t, "today_cost", q.SortBy)
	require.Equal(t, "desc", q.SortOrder)
	for _, filter := range []UserDailyStatsFilter{
		{Date: "2026-02-30"}, {Date: "2026-1-01"}, {Date: "bad"},
		{Date: "0000-01-01"}, {Date: "0001-01-01"},
		{SortBy: "balance; DROP TABLE users"}, {SortOrder: "DESC NULLS FIRST"},
		{Activity: "unknown"}, {Page: 1000001}, {Search: string(make([]rune, 201))},
	} {
		_, err := normalizeUserDailyStatsQuery(filter, now)
		require.Error(t, err, "%+v", filter)
	}
	for _, activity := range []string{"all", "active", "checkin", "liyu", "usage"} {
		_, err := normalizeUserDailyStatsQuery(UserDailyStatsFilter{Date: "2024-02-29", Activity: activity, SortBy: "welfare_balance", SortOrder: "asc"}, now)
		require.NoError(t, err)
	}
}

type dailyStatsStub struct {
	query UserDailyStatsQuery
	err   error
}

func (r *dailyStatsStub) GetDailyStats(ctx context.Context, q UserDailyStatsQuery) (*UserDailyStatsResponse, error) {
	r.query = q
	if _, ok := ctx.Deadline(); !ok {
		panic("missing query timeout")
	}
	return &UserDailyStatsResponse{}, r.err
}
func (r *dailyStatsStub) GetDailyActivities(_ context.Context, _ int64, q UserDailyStatsQuery) (*UserDailyActivitiesResponse, error) {
	r.query = q
	return &UserDailyActivitiesResponse{}, r.err
}

func TestUserDailyStatsServiceMetadataAndErrors(t *testing.T) {
	repo := &dailyStatsStub{}
	svc := NewUserDailyStatsService(repo)
	result, err := svc.Query(context.Background(), UserDailyStatsFilter{Date: "2026-11-13"})
	require.NoError(t, err)
	require.Equal(t, UserDailyStatsTimezone, result.Timezone)
	require.Equal(t, "2026-11-12", result.PreviousDate)
	require.False(t, result.GeneratedAt.IsZero())
	_, err = svc.Activities(context.Background(), 0, UserDailyStatsFilter{})
	require.Error(t, err)
	repo.err = errors.New("database unavailable")
	_, err = svc.Query(context.Background(), UserDailyStatsFilter{})
	require.ErrorIs(t, err, repo.err)
	_, err = svc.Activities(context.Background(), 1, UserDailyStatsFilter{})
	require.ErrorIs(t, err, repo.err)
}
