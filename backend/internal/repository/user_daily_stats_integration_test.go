//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUserDailyStatsIntegration(t *testing.T) {
	ctx := context.Background()
	loc, err := time.LoadLocation(service.UserDailyStatsTimezone)
	require.NoError(t, err)
	start := time.Date(2020, 2, 29, 0, 0, 0, 0, loc)
	marker := uuid.NewString()
	newUser := func(name string, balance service.WalletBalance) (int64, int64) {
		id, key := walletFixture(t, balance)
		_, err := integrationDB.Exec(`UPDATE users SET username=$2,created_at=$3 WHERE id=$1`, id, marker+name, start.Add(-48*time.Hour))
		require.NoError(t, err)
		return id, key
	}
	a, keyA := newUser("_a", service.WalletBalance{Paid: 15, Checkin: 2, Liyu: -1, Gift: 3})
	b, keyB := newUser("b", service.WalletBalance{Paid: 20})
	c, _ := newUser("c", service.WalletBalance{Checkin: 1})
	d, _ := newUser("d", service.WalletBalance{})
	account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: marker})
	usage := func(id, key int64, cost float64, at time.Time, billing int, paid, checkin, liyu, gift float64, settled bool) {
		rid := uuid.NewString()
		_, err := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,actual_cost,input_tokens,output_tokens,billing_type,created_at)
			VALUES($1,$2,$3,$4,'daily-stats', $5,10,20,$6,$7)`, id, key, account.ID, rid, cost, billing, at)
		require.NoError(t, err)
		if settled {
			_, err = integrationDB.Exec(`INSERT INTO wallet_transactions(user_id,event_type,operation_id,api_key_id,paid_delta,checkin_delta,liyu_delta,gift_delta,paid_after,checkin_after,liyu_after,gift_after,created_at)
				VALUES($1,'usage',$2,$3,$4,$5,$6,$7,0,0,0,0,$8)`, id, rid, key, -paid, -checkin, -liyu, -gift, at)
			require.NoError(t, err)
		}
	}
	usage(a, keyA, 9, start, 0, 6, 1, 1, 1, true)
	usage(a, keyA, 0.00000003, start.Add(time.Hour), 0, 0.00000002, 0.00000001, 0, 0, true)
	usage(a, keyA, 2, start.Add(2*time.Hour), 1, 0, 0, 0, 0, false)
	usage(a, keyA, 1, start.Add(3*time.Hour), 0, 0, 0, 0, 0, false)
	usage(a, keyA, 4, start.Add(-time.Nanosecond*1000), 0, 3, 1, 0, 0, true)
	usage(b, keyB, 20, start.Add(time.Hour), 0, 20, 0, 0, 0, true)
	usage(b, keyB, 100, start.Add(24*time.Hour), 0, 100, 0, 0, 0, true) // excluded end boundary
	usage(b, keyB, 100, start.Add(-24*time.Hour-time.Second), 0, 100, 0, 0, 0, true)
	_, err = integrationDB.Exec(`INSERT INTO checkin_records(user_id,checkin_date,reward_amount,base_reward_amount,bonus_delta_amount,bonus_status,bonus_played_at,created_at)
		VALUES($1,'2020-02-29',2,1,1,'win',$3,$2),($4,'2020-02-29',.5,.5,0,'none',NULL,$2),($5,'2020-02-28',2,1,1,'win',$2,$6)`, a, start, start.Add(24*time.Hour), c, d, start.Add(-time.Hour))
	require.NoError(t, err)
	// Multiple draws must not multiply usage rows; a previous-day draw's bonus belongs to today.
	for i, date := range []string{"2020-02-29", "2020-02-29", "2020-02-28"} {
		var bonusAt any
		adjustment := 0
		if i == 2 {
			bonusAt = start.Add(time.Hour)
			adjustment = 2
		}
		_, err = integrationDB.Exec(`INSERT INTO zenxiang_liyu_records(request_id,user_id,play_date,ticket_amount,reward_amount,user_net_amount,system_revenue,system_expense,system_profit,
			prize_name_snapshot,probability_snapshot,balance_before,balance_after_ticket,balance_after_reward,lucky_coin_adjustment,lucky_coin_played_at)
			VALUES($1,$2,$3,5,2,-3,5,2,3,'daily prize',100,0,0,2,$4,$5)`, uuid.NewString(), a, date, adjustment, bonusAt)
		require.NoError(t, err)
	}
	_, err = integrationDB.Exec(`UPDATE users SET deleted_at=NOW() WHERE id=$1`, c)
	require.NoError(t, err)
	svc := service.NewUserDailyStatsService(NewUserDailyStatsRepository(integrationDB))
	query := service.UserDailyStatsFilter{Date: "2020-02-29", Search: marker, PageSize: 100}
	result, err := svc.Query(ctx, query)
	require.NoError(t, err)
	require.EqualValues(t, 4, result.Total)
	require.Len(t, result.Items, 4)
	require.Equal(t, b, result.Items[0].UserID)
	require.EqualValues(t, 1, *result.Items[0].SpendingRank)
	row := result.Items[1]
	require.Equal(t, a, row.UserID)
	require.EqualValues(t, 2, *row.SpendingRank)
	require.InDelta(t, 12.00000003, row.TodayUsage.TotalCost, 1e-10)
	require.InDelta(t, 6.00000002, row.TodayUsage.PaidCost, 1e-10)
	require.InDelta(t, 3.00000001, row.TodayUsage.WelfareCost, 1e-10)
	require.EqualValues(t, 1, row.TodayUsage.LiyuCost)
	require.EqualValues(t, 1, row.TodayUsage.GiftCost)
	require.EqualValues(t, 2, row.TodayUsage.SubscriptionCost)
	require.EqualValues(t, 1, row.TodayUsage.UnclassifiedCost)
	require.EqualValues(t, 4, row.TodayUsage.Requests)
	require.EqualValues(t, 120, row.TodayUsage.Tokens)
	require.EqualValues(t, 4, row.PreviousUsage.TotalCost)
	require.EqualValues(t, 3, row.PreviousUsage.PaidCost)
	require.EqualValues(t, 1, row.Activity.CheckinNet, "tomorrow's bonus must be excluded")
	require.EqualValues(t, 2, row.Activity.LiyuDraws)
	require.EqualValues(t, 4, row.Activity.LiyuReward)
	require.EqualValues(t, 2, row.Activity.LiyuLucky)
	require.EqualValues(t, 6, row.Activity.LiyuNet)
	require.EqualValues(t, 10, row.Activity.LiyuTicketFaceValue)
	require.EqualValues(t, 15, row.Balance)
	require.EqualValues(t, 4, row.WelfareBalance, "negative welfare must remain visible")
	require.EqualValues(t, 2, result.Summary.SpendingUsers)
	require.EqualValues(t, 3, result.Summary.CheckinUsers)
	require.EqualValues(t, 1, result.Summary.LiyuUsers)
	require.InDelta(t, 32.00000003, result.Summary.TodayUsage.TotalCost, 1e-10)
	require.Nil(t, result.Items[2].SpendingRank)
	var deleted bool
	for _, item := range result.Items {
		if item.UserID == c {
			deleted = item.Deleted
		}
	}
	require.True(t, deleted)
	query.Search = marker + "_a"
	filtered, err := svc.Query(ctx, query)
	require.NoError(t, err)
	require.EqualValues(t, 1, filtered.Total)
	require.EqualValues(t, 2, *filtered.Items[0].SpendingRank, "search must preserve global ranking")
	query.Page = 2
	empty, err := svc.Query(ctx, query)
	require.NoError(t, err)
	require.Empty(t, empty.Items)
	require.EqualValues(t, 1, empty.Total)
	require.Equal(t, filtered.Summary, empty.Summary)
	query.Page = 1
	query.Search = marker
	query.Activity = "checkin"
	filtered, err = svc.Query(ctx, query)
	require.NoError(t, err)
	require.EqualValues(t, 3, filtered.Total, "include cross-day bonus participants")
	query.Activity = "usage"
	filtered, err = svc.Query(ctx, query)
	require.NoError(t, err)
	require.EqualValues(t, 2, filtered.Total)
	for _, sort := range []string{"today_cost", "previous_cost", "checkin_net", "liyu_net", "balance", "welfare_balance", "requests"} {
		query.Activity = "all"
		query.SortBy = sort
		query.SortOrder = "asc"
		query.PageSize = 1
		page, err := svc.Query(ctx, query)
		require.NoError(t, err, sort)
		require.Len(t, page.Items, 1)
		require.EqualValues(t, 4, page.Total)
		require.InDelta(t, 32.00000003, page.Summary.TodayUsage.TotalCost, 1e-10)
	}
	literal := marker + `%_\literal`
	_, err = integrationDB.Exec(`UPDATE users SET username=$2 WHERE id=$1`, c, literal)
	require.NoError(t, err)
	query.Search = literal
	literalResult, err := svc.Query(ctx, query)
	require.NoError(t, err)
	require.EqualValues(t, 1, literalResult.Total, "wildcard characters must match literally")
	require.Equal(t, c, literalResult.Items[0].UserID)
	details, err := svc.Activities(ctx, a, service.UserDailyStatsFilter{Date: query.Date, PageSize: 100})
	require.NoError(t, err)
	require.EqualValues(t, 4, details.Total)
	var sum float64
	for _, event := range details.Items {
		sum += event.Amount
		require.NotEmpty(t, event.Kind)
	}
	require.EqualValues(t, 7, sum)
	details, err = svc.Activities(ctx, a, service.UserDailyStatsFilter{Date: query.Date, Page: 10, PageSize: 100})
	require.NoError(t, err)
	require.Empty(t, details.Items)
	require.EqualValues(t, 4, details.Total)
	empty, err = svc.Query(ctx, service.UserDailyStatsFilter{Date: "1900-01-01"})
	require.NoError(t, err)
	require.Empty(t, empty.Items)
	require.Zero(t, empty.Summary.Users)
	require.Zero(t, empty.Summary.TodayUsage.TotalCost)
}
