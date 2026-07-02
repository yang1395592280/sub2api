package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sub2APICheckinRepoStub struct {
	AccountRepository
	account             *Account
	updatedExtra        map[string]any
	updatedCredentials  map[string]any
	updatedChannelPrice *float64
	updateExtraCalls    int
}

func (r *sub2APICheckinRepoStub) GetByID(context.Context, int64) (*Account, error) {
	if r.account == nil {
		return nil, ErrAccountNotFound
	}
	return r.account, nil
}

func (r *sub2APICheckinRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updateExtraCalls++
	r.updatedExtra = cloneAnyMap(updates)
	if r.account.Extra == nil {
		r.account.Extra = map[string]any{}
	}
	for k, v := range updates {
		r.account.Extra[k] = v
	}
	return nil
}

func (r *sub2APICheckinRepoStub) BulkUpdate(_ context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	if len(ids) == 0 || r.account == nil || ids[0] != r.account.ID {
		return 0, nil
	}
	if len(updates.Extra) > 0 {
		r.updateExtraCalls++
		r.updatedExtra = cloneAnyMap(updates.Extra)
		if r.account.Extra == nil {
			r.account.Extra = map[string]any{}
		}
		for k, v := range updates.Extra {
			r.account.Extra[k] = v
		}
	}
	if len(updates.Credentials) > 0 {
		if r.account.Credentials == nil {
			r.account.Credentials = map[string]any{}
		}
		for k, v := range updates.Credentials {
			r.account.Credentials[k] = v
		}
		r.updatedCredentials = cloneAnyMap(r.account.Credentials)
	}
	if updates.ChannelPrice != nil {
		channelPrice := *updates.ChannelPrice
		r.updatedChannelPrice = &channelPrice
		r.account.ChannelPrice = &channelPrice
	}
	return 1, nil
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func TestSub2APICheckinServiceScheduleWithinWindow(t *testing.T) {
	repo := &sub2APICheckinRepoStub{
		account: &Account{
			ID:       42,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                    "https://ai.clol.site",
				"api_key":                     "sk-upstream",
				"upstream_admin_type":         "sub2api",
				"upstream_admin_access_token": "admin-token",
				"upstream_checkin_enabled":    true,
				"upstream_checkin_url":        "/api/v1/user/checkin",
				"upstream_checkin_start_time": "08:00",
				"upstream_checkin_end_time":   "10:30",
			},
		},
	}
	loc := time.FixedZone("CST", 8*3600)
	svc := NewSub2APICheckinService(repo, nil, loc)
	next, err := svc.planNextRunForDate(time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), "08:00", "10:30")
	require.NoError(t, err)
	require.False(t, next.Before(time.Date(2026, 7, 2, 8, 0, 0, 0, loc)))
	require.True(t, next.Before(time.Date(2026, 7, 2, 10, 30, 1, 0, loc)))
}

func TestSub2APICheckinServiceTreatsAlreadyCheckedInAsSuccess(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/user/checkin", r.URL.Path)
		require.Equal(t, "Bearer admin-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"checked_in":true,"checked_in_at":"2026-07-02T08:37:12+08:00","reward_amount":10,"balance":89.5}}`))
	}))
	defer srv.Close()

	repo := &sub2APICheckinRepoStub{
		account: &Account{
			ID:       42,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                    srv.URL + "/v1",
				"api_key":                     "sk-upstream",
				"upstream_admin_type":         "sub2api",
				"upstream_admin_access_token": "admin-token",
				"upstream_checkin_enabled":    true,
				"upstream_checkin_url":        "/api/v1/user/checkin",
			},
		},
	}

	svc := NewSub2APICheckinService(repo, nil, loc)
	svc.client = srv.Client()
	svc.clock = func() time.Time {
		return time.Date(2026, 7, 2, 8, 40, 0, 0, loc)
	}

	account, err := svc.RefreshNow(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, Sub2APICheckinStatusSuccess, repo.updatedExtra["upstream_checkin_status"])
	require.Equal(t, "2026-07-02", repo.updatedExtra["upstream_checkin_last_success_date"])
	require.Equal(t, 10.0, repo.updatedExtra["upstream_checkin_reward_amount"])
	require.Equal(t, 89.5, repo.updatedExtra["upstream_checkin_balance"])
	require.Equal(t, "", repo.updatedExtra["upstream_checkin_error"])
	require.Equal(t, 0, repo.updatedExtra["upstream_checkin_retry_count"])
	require.Equal(t, "2026-07-02", account.GetExtraString("upstream_checkin_last_success_date"))
}

func TestSub2APICheckinServiceRetryCountResetsPerDay(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()

	repo := &sub2APICheckinRepoStub{
		account: &Account{
			ID:       43,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                    srv.URL + "/v1",
				"api_key":                     "sk-upstream",
				"upstream_admin_type":         "sub2api",
				"upstream_admin_access_token": "admin-token",
				"upstream_checkin_enabled":    true,
				"upstream_checkin_url":        "/api/v1/user/checkin",
			},
			Extra: map[string]any{
				"upstream_checkin_retry_date":  "2026-07-02",
				"upstream_checkin_retry_count": 3,
			},
		},
	}

	svc := NewSub2APICheckinService(repo, nil, loc)
	svc.client = srv.Client()
	now := time.Date(2026, 7, 3, 9, 0, 0, 0, loc)
	svc.clock = func() time.Time { return now }

	_, err := svc.RefreshNow(context.Background(), 43)
	require.NoError(t, err)
	require.Equal(t, Sub2APICheckinStatusError, repo.updatedExtra["upstream_checkin_status"])
	require.Equal(t, "2026-07-03", repo.updatedExtra["upstream_checkin_retry_date"])
	require.Equal(t, 1, repo.updatedExtra["upstream_checkin_retry_count"])
	nextRun, err := time.Parse(time.RFC3339, repo.updatedExtra["upstream_checkin_next_run_at"].(string))
	require.NoError(t, err)
	require.False(t, nextRun.Before(now.Add(10*time.Minute)))
	require.True(t, nextRun.Before(now.Add(30*time.Minute+time.Second)))
}

func TestSub2APICheckinServiceReconcileSkipsSchedulingAfterRetryCapSameDay(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 7, 2, 9, 30, 0, 0, loc)
	repo := &sub2APICheckinRepoStub{
		account: &Account{
			ID:       46,
			Status:   StatusActive,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                    "https://ai.clol.site",
				"api_key":                     "sk-upstream",
				"upstream_admin_type":         "sub2api",
				"upstream_admin_access_token": "admin-token",
				"upstream_checkin_enabled":    true,
				"upstream_checkin_start_time": "08:00",
				"upstream_checkin_end_time":   "10:30",
			},
			Extra: map[string]any{
				"upstream_checkin_retry_date":  "2026-07-02",
				"upstream_checkin_retry_count": 3,
				"upstream_checkin_next_run_at": "",
			},
		},
	}

	svc := NewSub2APICheckinService(repo, nil, loc)
	svc.clock = func() time.Time { return now }

	err := svc.reconcileAccount(context.Background(), repo.account, now)
	require.NoError(t, err)
	require.Equal(t, 0, repo.updateExtraCalls)
	require.Nil(t, repo.updatedExtra)
	require.Equal(t, "", repo.account.GetExtraString("upstream_checkin_next_run_at"))
	require.Equal(t, 3, repo.account.getExtraInt("upstream_checkin_retry_count"))
}

func TestSub2APICheckinServiceReconcileExecutesPlannedFinalRetryWhenDue(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 7, 2, 9, 30, 0, 0, loc)
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/user/checkin", r.URL.Path)
		require.Equal(t, "Bearer admin-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"checked_in":true}}`))
	}))
	defer srv.Close()

	repo := &sub2APICheckinRepoStub{
		account: &Account{
			ID:       47,
			Status:   StatusActive,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                    srv.URL + "/v1",
				"api_key":                     "sk-upstream",
				"upstream_admin_type":         "sub2api",
				"upstream_admin_access_token": "admin-token",
				"upstream_checkin_enabled":    true,
				"upstream_checkin_url":        "/api/v1/user/checkin",
				"upstream_checkin_start_time": "08:00",
				"upstream_checkin_end_time":   "10:30",
			},
			Extra: map[string]any{
				"upstream_checkin_retry_date":  "2026-07-02",
				"upstream_checkin_retry_count": 3,
				"upstream_checkin_next_run_at": now.Add(-5 * time.Minute).Format(time.RFC3339),
			},
		},
	}

	svc := NewSub2APICheckinService(repo, nil, loc)
	svc.client = srv.Client()
	svc.clock = func() time.Time { return now }

	err := svc.reconcileAccount(context.Background(), repo.account, now)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)
	require.Equal(t, Sub2APICheckinStatusSuccess, repo.updatedExtra["upstream_checkin_status"])
}

func TestSub2APICheckinServiceFinalRetryFailureDoesNotScheduleNewRun(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 7, 2, 9, 30, 0, 0, loc)
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		require.Equal(t, http.MethodPost, r.Method)
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()

	repo := &sub2APICheckinRepoStub{
		account: &Account{
			ID:       48,
			Status:   StatusActive,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                    srv.URL + "/v1",
				"api_key":                     "sk-upstream",
				"upstream_admin_type":         "sub2api",
				"upstream_admin_access_token": "admin-token",
				"upstream_checkin_enabled":    true,
				"upstream_checkin_url":        "/api/v1/user/checkin",
			},
			Extra: map[string]any{
				"upstream_checkin_retry_date":  "2026-07-02",
				"upstream_checkin_retry_count": 3,
				"upstream_checkin_next_run_at": now.Add(-5 * time.Minute).Format(time.RFC3339),
			},
		},
	}

	svc := NewSub2APICheckinService(repo, nil, loc)
	svc.client = srv.Client()
	svc.clock = func() time.Time { return now }

	err := svc.reconcileAccount(context.Background(), repo.account, now)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)
	require.Equal(t, Sub2APICheckinStatusError, repo.updatedExtra["upstream_checkin_status"])
	require.Equal(t, 3, repo.updatedExtra["upstream_checkin_retry_count"])
	require.Equal(t, "", repo.updatedExtra["upstream_checkin_next_run_at"])
}

func TestSub2APICheckinServiceAuthFallbackOrder(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)

	t.Run("refresh token falls back to access token before login", func(t *testing.T) {
		loginCalled := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/auth/refresh":
				http.Error(w, "refresh failed", http.StatusUnauthorized)
			case "/api/v1/auth/login":
				loginCalled = true
				http.Error(w, "login should not run", http.StatusInternalServerError)
			case "/api/v1/user/checkin":
				require.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
				_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"checked_in":true}}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		repo := &sub2APICheckinRepoStub{
			account: &Account{
				ID:       44,
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"base_url":                     srv.URL + "/v1",
					"api_key":                      "sk-upstream",
					"upstream_admin_type":          "sub2api",
					"upstream_admin_refresh_token": "rt-admin",
					"upstream_admin_access_token":  "access-token",
					"upstream_admin_email":         "admin@example.com",
					"upstream_admin_password":      "secret",
				},
			},
		}

		svc := NewSub2APICheckinService(repo, nil, loc)
		svc.client = srv.Client()
		svc.clock = func() time.Time { return time.Date(2026, 7, 2, 9, 0, 0, 0, loc) }

		_, err := svc.RefreshNow(context.Background(), 44)
		require.NoError(t, err)
		require.False(t, loginCalled)
	})

	t.Run("refresh token falls back to login when access token is absent", func(t *testing.T) {
		refreshCalled := false
		loginCalled := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/auth/refresh":
				refreshCalled = true
				http.Error(w, "refresh failed", http.StatusUnauthorized)
			case "/api/v1/auth/login":
				loginCalled = true
				var payload map[string]string
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
				require.Equal(t, "admin@example.com", payload["email"])
				require.Equal(t, "secret", payload["password"])
				_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"login-token","token_type":"Bearer"}}`))
			case "/api/v1/user/checkin":
				require.Equal(t, "Bearer login-token", r.Header.Get("Authorization"))
				_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"checked_in":true}}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		repo := &sub2APICheckinRepoStub{
			account: &Account{
				ID:       45,
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"base_url":                     srv.URL + "/v1",
					"api_key":                      "sk-upstream",
					"upstream_admin_type":          "sub2api",
					"upstream_admin_refresh_token": "rt-admin",
					"upstream_admin_email":         "admin@example.com",
					"upstream_admin_password":      "secret",
				},
			},
		}

		svc := NewSub2APICheckinService(repo, nil, loc)
		svc.client = srv.Client()
		svc.clock = func() time.Time { return time.Date(2026, 7, 2, 9, 0, 0, 0, loc) }

		_, err := svc.RefreshNow(context.Background(), 45)
		require.NoError(t, err)
		require.True(t, refreshCalled)
		require.True(t, loginCalled)
	})
}
