package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userDailyStatsRepository struct{ db *sql.DB }

func NewUserDailyStatsRepository(db *sql.DB) service.UserDailyStatsRepository {
	return &userDailyStatsRepository{db: db}
}

// Bonuses are attributed to their settlement day, even when the original draw
// happened earlier. Ticket face values are vouchers, not a wallet cash debit.
func dailyActivityEventsSQL(dateParam, userFilter string) string {
	return fmt.Sprintf(`
		SELECT user_id, 'checkin' AS kind, id AS record_id, base_reward_amount AS amount,
			0::numeric AS ticket_face_value, '' AS outcome, '' AS description, created_at
		FROM checkin_records WHERE checkin_date=%s %s
		UNION ALL
		SELECT user_id, 'checkin_bonus', id, bonus_delta_amount, 0, bonus_status, '', bonus_played_at
		FROM checkin_records WHERE bonus_played_at >= $1 AND bonus_played_at < $2 %s
		UNION ALL
		SELECT user_id, 'liyu_draw', id, reward_amount, ticket_amount, '', prize_name_snapshot, created_at
		FROM zenxiang_liyu_records WHERE play_date=%s::date %s
		UNION ALL
		SELECT user_id, 'liyu_lucky', id, lucky_coin_adjustment, 0, lucky_coin_outcome, prize_name_snapshot, lucky_coin_played_at
		FROM zenxiang_liyu_records WHERE lucky_coin_played_at >= $1 AND lucky_coin_played_at < $2 %s
		UNION ALL
		SELECT user_id, 'liyu_guess', id, guess_size_adjustment, 0,
			CASE WHEN guess_size_choice='skip' THEN 'skip' WHEN guess_size_won THEN 'win' ELSE 'lose' END,
			prize_name_snapshot, guess_size_played_at
		FROM zenxiang_liyu_records WHERE guess_size_played_at >= $1 AND guess_size_played_at < $2 %s
		UNION ALL
		SELECT user_id, 'admin_gift', id, gift_delta, 0, '', notes, created_at
		FROM wallet_transactions WHERE event_type='admin_credit' AND gift_delta<>0 AND created_at >= $1 AND created_at < $2 %s
		UNION ALL
		SELECT user_id, event_type, id, checkin_delta+liyu_delta+gift_delta, 0, '', notes, created_at
		FROM wallet_transactions WHERE event_type IN ('admin_refund','checkin_clear','checkin_clear_refund','checkin_restore')
			AND checkin_delta+liyu_delta+gift_delta<>0 AND created_at >= $1 AND created_at < $2 %s
	`, dateParam, userFilter, userFilter, dateParam, userFilter, userFilter, userFilter, userFilter, userFilter)
}

var dailyUsageFields = []string{"total_cost", "paid_cost", "welfare_cost", "checkin_cost", "liyu_cost", "gift_cost", "subscription_cost", "unclassified_cost", "requests", "tokens"}
var dailyActivityFields = []string{"event_count", "checkin_events", "checkin_count", "checkin_base", "checkin_bonus", "checkin_net", "liyu_draws", "liyu_events", "liyu_reward", "liyu_lucky", "liyu_guess", "liyu_net", "liyu_ticket_face_value", "gift_credit", "welfare_adjustment"}

func dailyStatsJSONObject(alias string, fields []string) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, fmt.Sprintf("'%s', COALESCE(%s.%s,0)", field, alias, field))
	}
	return "jsonb_build_object(" + strings.Join(parts, ",") + ")"
}

func dailyStatsSummaryJSON(column string, fields []string) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, fmt.Sprintf("'%s', COALESCE(SUM((%s->>'%s')::numeric),0)", field, column, field))
	}
	return "jsonb_build_object(" + strings.Join(parts, ",") + ")"
}

func dailyStatsOrder(sortBy, order string) string {
	columns := map[string]string{
		"today_cost":    "(today_usage->>'total_cost')::numeric",
		"previous_cost": "(previous_usage->>'total_cost')::numeric",
		"checkin_net":   "(activity->>'checkin_net')::numeric",
		"liyu_net":      "(activity->>'liyu_net')::numeric",
		"balance":       "balance", "welfare_balance": "welfare_balance",
		"requests": "(today_usage->>'requests')::bigint",
	}
	column, ok := columns[sortBy]
	if !ok {
		column = columns["today_cost"]
	}
	direction := "DESC"
	if order == "asc" {
		direction = "ASC"
	}
	return column + " " + direction + ", (today_usage->>'tokens')::bigint DESC, user_id ASC"
}

func (r *userDailyStatsRepository) GetDailyStats(ctx context.Context, q service.UserDailyStatsQuery) (*service.UserDailyStatsResponse, error) {
	// All sources are aggregated separately before joining users. The final
	// statement gives ranking, filtered totals and rows the same MVCC snapshot.
	query := fmt.Sprintf(`WITH daily_usage AS (
		SELECT ul.user_id, (ul.created_at >= $1) AS selected_day,
			COALESCE(SUM(ul.actual_cost),0) AS total_cost,
			COALESCE(-SUM(w.paid_delta),0) AS paid_cost,
			COALESCE(-SUM(w.checkin_delta+w.liyu_delta+w.gift_delta),0) AS welfare_cost,
			COALESCE(-SUM(w.checkin_delta),0) AS checkin_cost,
			COALESCE(-SUM(w.liyu_delta),0) AS liyu_cost,
			COALESCE(-SUM(w.gift_delta),0) AS gift_cost,
			COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.billing_type=1),0) AS subscription_cost,
			COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.billing_type=0 AND w.id IS NULL),0) AS unclassified_cost,
			COUNT(*) AS requests,
			COALESCE(SUM(ul.input_tokens::bigint+ul.output_tokens+ul.cache_creation_tokens+ul.cache_read_tokens),0) AS tokens
		FROM usage_logs ul LEFT JOIN wallet_transactions w ON ul.billing_type=0 AND w.user_id=ul.user_id
			AND w.api_key_id=ul.api_key_id AND w.operation_id=ul.request_id AND w.event_type='usage'
		WHERE ul.created_at >= $3 AND ul.created_at < $2
		GROUP BY ul.user_id, (ul.created_at >= $1)
	), activity_events AS (%s), daily_activity AS (
		SELECT user_id, COUNT(*) AS event_count,
			COUNT(*) FILTER (WHERE kind IN ('checkin','checkin_bonus')) AS checkin_events,
			COUNT(*) FILTER (WHERE kind='checkin') AS checkin_count,
			COALESCE(SUM(amount) FILTER (WHERE kind='checkin'),0) AS checkin_base,
			COALESCE(SUM(amount) FILTER (WHERE kind='checkin_bonus'),0) AS checkin_bonus,
			COALESCE(SUM(amount) FILTER (WHERE kind IN ('checkin','checkin_bonus')),0) AS checkin_net,
			COUNT(*) FILTER (WHERE kind='liyu_draw') AS liyu_draws,
			COUNT(*) FILTER (WHERE kind IN ('liyu_draw','liyu_lucky','liyu_guess')) AS liyu_events,
			COALESCE(SUM(amount) FILTER (WHERE kind='liyu_draw'),0) AS liyu_reward,
			COALESCE(SUM(amount) FILTER (WHERE kind='liyu_lucky'),0) AS liyu_lucky,
			COALESCE(SUM(amount) FILTER (WHERE kind='liyu_guess'),0) AS liyu_guess,
			COALESCE(SUM(amount) FILTER (WHERE kind IN ('liyu_draw','liyu_lucky','liyu_guess')),0) AS liyu_net,
			COALESCE(SUM(ticket_face_value),0) AS liyu_ticket_face_value,
			COALESCE(SUM(amount) FILTER (WHERE kind='admin_gift'),0) AS gift_credit,
			COALESCE(SUM(amount) FILTER (WHERE kind NOT IN ('checkin','checkin_bonus','liyu_draw','liyu_lucky','liyu_guess','admin_gift')),0) AS welfare_adjustment
		FROM activity_events GROUP BY user_id
	), base AS (
		SELECT u.id AS user_id, u.email, COALESCE(u.username,'') AS username, u.status,
			u.deleted_at IS NOT NULL AS deleted, u.balance, u.checkin_balance, u.liyu_balance, u.gift_balance,
			u.checkin_balance+u.liyu_balance+u.gift_balance AS welfare_balance, u.frozen_balance, u.total_recharged,
			%s AS today_usage, %s AS previous_usage, %s AS activity
		FROM users u LEFT JOIN daily_usage today ON today.user_id=u.id AND today.selected_day
			LEFT JOIN daily_usage previous ON previous.user_id=u.id AND NOT previous.selected_day
			LEFT JOIN daily_activity a ON a.user_id=u.id
		WHERE u.created_at < $2
	), ranked AS (
		SELECT user_id,email,username,status,deleted,balance,checkin_balance,liyu_balance,gift_balance,welfare_balance,
			frozen_balance,total_recharged,today_usage,previous_usage,activity,
			CASE WHEN (today_usage->>'total_cost')::numeric > 0 THEN ROW_NUMBER() OVER (
				ORDER BY (today_usage->>'total_cost')::numeric DESC, (today_usage->>'tokens')::bigint DESC, user_id ASC
			) END AS spending_rank
		FROM base
	), filtered AS (
		SELECT user_id,email,username,status,deleted,balance,checkin_balance,liyu_balance,gift_balance,welfare_balance,
			frozen_balance,total_recharged,today_usage,previous_usage,activity,spending_rank
		FROM ranked WHERE ($5='' OR email ILIKE $5 OR username ILIKE $5 OR user_id=$6)
			AND ($7='all'
				OR ($7='active' AND ((activity->>'event_count')::bigint>0 OR (today_usage->>'requests')::bigint>0))
				OR ($7='checkin' AND (activity->>'checkin_events')::bigint>0)
				OR ($7='liyu' AND (activity->>'liyu_events')::bigint>0)
				OR ($7='usage' AND (today_usage->>'requests')::bigint>0))
	), summary AS (
		SELECT jsonb_build_object('users',COUNT(*),
			'active_users',COUNT(*) FILTER (WHERE (activity->>'event_count')::bigint>0 OR (today_usage->>'requests')::bigint>0),
			'checkin_users',COUNT(*) FILTER (WHERE (activity->>'checkin_events')::bigint>0),
			'liyu_users',COUNT(*) FILTER (WHERE (activity->>'liyu_events')::bigint>0),
			'spending_users',COUNT(*) FILTER (WHERE (today_usage->>'total_cost')::numeric>0),
			'balance',COALESCE(SUM(balance),0),'welfare_balance',COALESCE(SUM(welfare_balance),0),
			'today_usage',%s,'previous_usage',%s,'activity',%s) AS data FROM filtered
	), page_rows AS (
		SELECT user_id,email,username,status,deleted,balance,checkin_balance,liyu_balance,gift_balance,welfare_balance,
			frozen_balance,total_recharged,today_usage,previous_usage,activity,spending_rank
		FROM filtered ORDER BY %s LIMIT $8 OFFSET $9
	)
	SELECT COALESCE((SELECT jsonb_agg(to_jsonb(page_rows)) FROM page_rows),'[]'::jsonb),
		(SELECT data FROM summary),(SELECT COUNT(*) FROM filtered)`, dailyActivityEventsSQL("$4", ""),
		dailyStatsJSONObject("today", dailyUsageFields), dailyStatsJSONObject("previous", dailyUsageFields), dailyStatsJSONObject("a", dailyActivityFields),
		dailyStatsSummaryJSON("today_usage", dailyUsageFields), dailyStatsSummaryJSON("previous_usage", dailyUsageFields), dailyStatsSummaryJSON("activity", dailyActivityFields),
		dailyStatsOrder(q.SortBy, q.SortOrder))
	pattern := ""
	if q.Search != "" {
		pattern = "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(q.Search) + "%"
	}
	searchID, _ := strconv.ParseInt(q.Search, 10, 64)
	var itemsJSON, summaryJSON []byte
	result := &service.UserDailyStatsResponse{Page: q.Page, PageSize: q.PageSize}
	err := r.db.QueryRowContext(ctx, query, q.Start, q.End, q.PreviousStart, q.Date, pattern, searchID, q.Activity, q.PageSize, (q.Page-1)*q.PageSize).
		Scan(&itemsJSON, &summaryJSON, &result.Total)
	if err != nil {
		return nil, fmt.Errorf("query user daily statistics: %w", err)
	}
	if err = json.Unmarshal(itemsJSON, &result.Items); err != nil {
		return nil, fmt.Errorf("decode user daily rows: %w", err)
	}
	if err = json.Unmarshal(summaryJSON, &result.Summary); err != nil {
		return nil, fmt.Errorf("decode user daily summary: %w", err)
	}
	result.Pages = int((result.Total + int64(q.PageSize) - 1) / int64(q.PageSize))
	return result, nil
}

func (r *userDailyStatsRepository) GetDailyActivities(ctx context.Context, userID int64, q service.UserDailyStatsQuery) (*service.UserDailyActivitiesResponse, error) {
	query := `WITH activity_events AS (` + dailyActivityEventsSQL("$3", "AND user_id=$4") + `), page_rows AS (
		SELECT kind,record_id,amount,ticket_face_value,outcome,description,created_at FROM activity_events
		ORDER BY created_at DESC,kind,record_id DESC LIMIT $5 OFFSET $6
	) SELECT COALESCE((SELECT jsonb_agg(to_jsonb(page_rows)) FROM page_rows),'[]'::jsonb), (SELECT COUNT(*) FROM activity_events)`
	var itemsJSON []byte
	result := &service.UserDailyActivitiesResponse{Page: q.Page, PageSize: q.PageSize}
	err := r.db.QueryRowContext(ctx, query, q.Start, q.End, q.Date, userID, q.PageSize, (q.Page-1)*q.PageSize).Scan(&itemsJSON, &result.Total)
	if err != nil {
		return nil, fmt.Errorf("query daily activities for user %d: %w", userID, err)
	}
	if err = json.Unmarshal(itemsJSON, &result.Items); err != nil {
		return nil, fmt.Errorf("decode daily activities for user %d: %w", userID, err)
	}
	result.Pages = int((result.Total + int64(q.PageSize) - 1) / int64(q.PageSize))
	return result, nil
}
