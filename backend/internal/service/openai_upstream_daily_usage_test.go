package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUpstreamDailyUsageSub2APIRefresh(t *testing.T) {
	date := newUpstreamDailyUsage().Date
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			require.Equal(t, "Asia/Shanghai", r.URL.Query().Get("timezone"))
			require.Equal(t, "1", r.URL.Query().Get("days"))
			fmt.Fprintf(w, `{"remaining":100,"unit":"USD","group_name":"Codex","group_rate_multiplier":0.3,"daily_usage":[{"date":%q,"actual_cost":6,"cost":20}]}`, date)
		case "/api/v1/usage/stats":
			require.Equal(t, "Bearer test-access", r.Header.Get("Authorization"))
			require.Equal(t, date, r.URL.Query().Get("start_date"))
			require.Equal(t, date, r.URL.Query().Get("end_date"))
			fmt.Fprint(w, `{"code":0,"data":{"total_actual_cost":9}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	repo := &openAIUpstreamBalanceRepoStub{account: &Account{
		ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, UpstreamRechargeRatio: 2,
		Credentials: map[string]any{"base_url": srv.URL, "api_key": "sk-current", "upstream_admin_type": "sub2api", "upstream_admin_access_token": "test-access"},
	}}
	account, err := NewOpenAIUpstreamBalanceService(repo, srv.Client()).Refresh(context.Background(), 42)
	require.NoError(t, err)
	usage := account.Extra["upstream_balance_daily_usage"].(*UpstreamDailyUsage)
	require.Equal(t, "ok", usage.Status)
	require.NotNil(t, usage.AccountCost)
	require.Equal(t, 6.0, *usage.KeyCost)
	require.Equal(t, 9.0, *usage.AccountCost)
	require.Equal(t, 3.0, *usage.CashCost)
	require.Equal(t, 50.0, account.Extra["upstream_balance_remaining"])
}

func TestUpstreamDailyUsageSub2APIEmptyAndUnavailable(t *testing.T) {
	for _, tc := range []struct{ name, payload, status string }{
		{"empty", `{"daily_usage":[]}`, "ok"},
		{"missing", `{}`, "unavailable"},
		{"bad_amount", `{"daily_usage":[{"date":"` + newUpstreamDailyUsage().Date + `"}]}`, "unavailable"},
		{"wrong_day", `{"daily_usage":[{"date":"2000-01-01","actual_cost":9}]}`, "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var payload map[string]any
			require.NoError(t, json.Unmarshal([]byte(tc.payload), &payload))
			snapshot := OpenAIUpstreamBalanceSnapshot{}
			NewOpenAIUpstreamBalanceService(nil, nil).enrichSub2APIDailyUsage(context.Background(), &Account{}, "", "", payload, &snapshot)
			require.Equal(t, tc.status, snapshot.DailyUsage.Status)
			if tc.status == "ok" {
				require.Equal(t, 0.0, *snapshot.DailyUsage.KeyCost)
			} else {
				require.Nil(t, snapshot.DailyUsage.KeyCost)
			}
		})
	}
}

func TestUpstreamDailyUsageSub2APIManagementFallback(t *testing.T) {
	var keyQueries int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/keys":
			fmt.Fprint(w, `{"code":0,"data":{"items":[{"id":17,"key":"sk-current"},{"id":18,"key":"sk-other"}]}}`)
		case "/api/v1/usage/stats":
			if r.URL.Query().Get("api_key_id") != "" {
				require.Equal(t, "17", r.URL.Query().Get("api_key_id"))
				keyQueries++
				fmt.Fprint(w, `{"code":0,"data":{"total_actual_cost":2.5}}`)
			} else {
				fmt.Fprint(w, `{"code":0,"data":{"total_actual_cost":8}}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	snapshot := OpenAIUpstreamBalanceSnapshot{}
	account := &Account{Credentials: map[string]any{"upstream_admin_type": "sub2api", "upstream_admin_access_token": "test-access"}}
	NewOpenAIUpstreamBalanceService(nil, srv.Client()).enrichSub2APIDailyUsage(context.Background(), account, srv.URL, "sk-current", nil, &snapshot)
	require.Equal(t, "ok", snapshot.DailyUsage.Status)
	require.Equal(t, 2.5, *snapshot.DailyUsage.KeyCost)
	require.Equal(t, 8.0, *snapshot.DailyUsage.AccountCost)
	require.Equal(t, 1, keyQueries)
}

func TestUpstreamDailyUsageNewAPIPaginationAndTokenID(t *testing.T) {
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/status" {
			fmt.Fprint(w, `{"success":true,"data":{"quota_per_unit":250000}}`)
			return
		}
		require.Equal(t, "/api/log/self", r.URL.Path)
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		require.Equal(t, "test-access", r.Header.Get("Authorization"))
		require.Empty(t, r.URL.Query().Get("token_name"), "renamed tokens must remain included")
		start, err := strconv.ParseInt(r.URL.Query().Get("start_timestamp"), 10, 64)
		require.NoError(t, err)
		require.Equal(t, 0, time.Unix(start, 0).In(time.FixedZone("CST", 8*3600)).Hour())
		pages++
		items := []map[string]any{}
		if pages == 1 {
			for i := 0; i < 100; i++ {
				// Two keys with identical names must not be merged.
				items = append(items, map[string]any{"type": 2, "token_id": 17 + i%2, "token_name": "duplicate", "quota": 5000})
			}
		} else {
			items = append(items, map[string]any{"type": 2, "token_id": 17, "token_name": "old name", "quota": 500000})
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"total": 101, "items": items}})
	}))
	defer srv.Close()
	keyID := int64(17)
	snapshot := OpenAIUpstreamBalanceSnapshot{UpstreamKeyID: &keyID}
	NewOpenAIUpstreamBalanceService(nil, srv.Client()).enrichNewAPIDailyUsage(context.Background(), srv.URL, newAPIUserBalanceAuth{UserID: "9", AccessToken: "test-access"}, &snapshot)
	normalizeOpenAIUpstreamBalanceSnapshot(&snapshot, 2)
	require.Equal(t, 2, pages)
	require.Equal(t, "ok", snapshot.DailyUsage.Status)
	require.Equal(t, 3.0, *snapshot.DailyUsage.KeyCost)
	require.Equal(t, 4.0, *snapshot.DailyUsage.AccountCost)
	require.Equal(t, 1.5, *snapshot.DailyUsage.CashCost)
}

func TestUpstreamDailyUsageNewAPIFailuresNeverBecomeZero(t *testing.T) {
	for _, tc := range []struct{ name, body, status, errorCode string }{
		{"empty", `{"success":true,"data":{"total":0,"items":[]}}`, "ok", ""},
		{"denied", `{"success":false}`, "unavailable", "unavailable"},
		{"missing_id", `{"success":true,"data":{"total":1,"items":[{"type":2,"quota":5}]}}`, "unavailable", "incomplete"},
		{"missing_amount", `{"success":true,"data":{"total":1,"items":[{"type":2,"token_id":17}]}}`, "unavailable", "incomplete"},
		{"partial", `{"success":true,"data":{"total":5,"items":[]}}`, "unavailable", "incomplete"},
		{"limit", `{"success":true,"data":{"total":10001,"items":[]}}`, "unavailable", "limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
			defer srv.Close()
			keyID := int64(17)
			snapshot := OpenAIUpstreamBalanceSnapshot{UpstreamKeyID: &keyID}
			NewOpenAIUpstreamBalanceService(nil, srv.Client()).enrichNewAPIDailyUsage(context.Background(), srv.URL, newAPIUserBalanceAuth{}, &snapshot)
			require.Equal(t, tc.status, snapshot.DailyUsage.Status)
			require.Equal(t, tc.errorCode, snapshot.DailyUsage.Error)
			if tc.status == "ok" {
				require.Equal(t, 0.0, *snapshot.DailyUsage.KeyCost)
			} else {
				require.Nil(t, snapshot.DailyUsage.KeyCost)
				require.Nil(t, snapshot.DailyUsage.AccountCost)
			}
		})
	}
}

func TestUpstreamDailyUsageKeyMaskMatching(t *testing.T) {
	require.True(t, newAPIKeyMatchesAPIKey("abc********xyz", "sk-abc123xyz"))
	require.False(t, newAPIKeyMatchesAPIKey("********", "sk-other"))
	require.False(t, newAPIKeyMatchesAPIKey("abc****xyz", "sk-abc123no"))
}
