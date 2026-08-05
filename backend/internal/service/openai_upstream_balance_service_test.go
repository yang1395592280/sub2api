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

type openAIUpstreamBalanceRepoStub struct {
	AccountRepository
	account             *Account
	updatedExtra        map[string]any
	updatedCredentials  map[string]any
	updatedChannelPrice *float64
}

func (r *openAIUpstreamBalanceRepoStub) GetByID(context.Context, int64) (*Account, error) {
	if r.account == nil {
		return nil, ErrAccountNotFound
	}
	return r.account, nil
}

func (r *openAIUpstreamBalanceRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updatedExtra = make(map[string]any, len(updates))
	for k, v := range updates {
		r.updatedExtra[k] = v
	}
	if r.account.Extra == nil {
		r.account.Extra = map[string]any{}
	}
	for k, v := range updates {
		r.account.Extra[k] = v
	}
	return nil
}

func (r *openAIUpstreamBalanceRepoStub) BulkUpdate(_ context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	if len(ids) == 0 || r.account == nil || ids[0] != r.account.ID {
		return 0, nil
	}
	if len(updates.Extra) > 0 {
		r.updatedExtra = make(map[string]any, len(updates.Extra))
		for k, v := range updates.Extra {
			r.updatedExtra[k] = v
		}
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
		r.updatedCredentials = make(map[string]any, len(r.account.Credentials))
		for k, v := range r.account.Credentials {
			r.updatedCredentials[k] = v
		}
	}
	if updates.ChannelPrice != nil {
		channelPrice := *updates.ChannelPrice
		r.updatedChannelPrice = &channelPrice
		r.account.ChannelPrice = &channelPrice
	}
	return 1, nil
}

func TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIUsageRemaining(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"remaining":12.34,"unit":"USD","balance":99}`))
		case "/v1/sub2api/billing":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       9,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
			Extra: map[string]any{"existing": "kept"},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	account, err := svc.Refresh(context.Background(), 9)
	require.NoError(t, err)
	require.Equal(t, "sub2api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, 12.34, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, "USD", repo.updatedExtra["upstream_balance_unit"])
	require.Equal(t, "ok", repo.updatedExtra["upstream_balance_status"])
	require.Equal(t, "kept", account.Extra["existing"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIBillingFallbackWritesChannelPrice(t *testing.T) {
	billingCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"remaining":46.04182761,"unit":"USD"}`))
		case "/v1/sub2api/billing":
			billingCalls++
			require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"object":"sub2api.key_billing","schema_version":1,"billing_scope":"token","group_rate_multiplier":0.1,"resolved_rate_multiplier":0.1,"peak_rate_enabled":false,"effective_rate_multiplier":0.1,"observed_at":"2026-07-17T10:33:43.684117965Z"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       30,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra:    map[string]any{"upstream_group": "Walk AI Pro"},
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	account, err := svc.Refresh(context.Background(), 30)
	require.NoError(t, err)
	require.Equal(t, 1, billingCalls)
	require.Equal(t, 46.04182761, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, 0.1, repo.updatedExtra["upstream_group_rate_multiplier"])
	require.Equal(t, 0.1, repo.updatedExtra["upstream_effective_rate_multiplier"])
	require.Equal(t, "sub2api_billing", repo.updatedExtra["upstream_rate_source"])
	require.NotNil(t, repo.updatedChannelPrice)
	require.Equal(t, 0.1, *repo.updatedChannelPrice)
	require.NotNil(t, account.ChannelPrice)
	require.Equal(t, 0.1, *account.ChannelPrice)
	require.Equal(t, "Walk AI Pro", account.Extra["upstream_group"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_AppliesUpstreamRechargeRatio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/usage", r.URL.Path)
		_, _ = w.Write([]byte(`{"remaining":100,"unit":"USD","group_id":2,"group":{"id":2,"name":"Tenfold","rate_multiplier":10}}`))
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:                    32,
			Platform:              PlatformOpenAI,
			Type:                  AccountTypeAPIKey,
			UpstreamRechargeRatio: 10,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	account, err := svc.Refresh(context.Background(), 32)
	require.NoError(t, err)
	require.Equal(t, 10.0, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, 1.0, repo.updatedExtra["upstream_group_rate_multiplier"])
	require.NotNil(t, repo.updatedChannelPrice)
	require.Equal(t, 1.0, *repo.updatedChannelPrice)
	require.NotNil(t, account.ChannelPrice)
	require.Equal(t, 1.0, *account.ChannelPrice)
}

func TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIUsageGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/usage", r.URL.Path)
		require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"remaining":0.13404922,"unit":"USD","group_id":2,"group":{"id":2,"name":"GPT Plus","rate_multiplier":0.08}}`))
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       19,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			ChannelPrice: func() *float64 {
				price := 1.23
				return &price
			}(),
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 19)
	require.NoError(t, err)
	require.Equal(t, "sub2api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, 0.13404922, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, "GPT Plus", repo.updatedExtra["upstream_group"])
	require.Equal(t, int64(2), repo.updatedExtra["upstream_group_id"])
	require.Equal(t, 0.08, repo.updatedExtra["upstream_group_rate_multiplier"])
	require.NotNil(t, repo.updatedChannelPrice)
	require.Equal(t, 0.08, *repo.updatedChannelPrice)
}

func TestOpenAIUpstreamBalanceServiceRefresh_Sub2APINestedAPIKeyGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/usage", r.URL.Path)
		require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{
			"remaining":10,
			"unit":"USD",
			"api_key":{
				"id":374,
				"group_id":5,
				"group":{"id":5,"name":"codex-team low price","rate_multiplier":0.02}
			}
		}`))
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       31,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 31)
	require.NoError(t, err)
	require.Equal(t, "sub2api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, 10.0, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, "codex-team low price", repo.updatedExtra["upstream_group"])
	require.Equal(t, int64(5), repo.updatedExtra["upstream_group_id"])
	require.Equal(t, 0.02, repo.updatedExtra["upstream_group_rate_multiplier"])
	require.NotNil(t, repo.updatedChannelPrice)
	require.Equal(t, 0.02, *repo.updatedChannelPrice)
}

func TestOpenAIUpstreamBalanceServiceRefresh_AnthropicAPIKeySub2APIUsageGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/usage", r.URL.Path)
		require.Equal(t, "Bearer sk-ant-upstream", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"remaining":6.78,"unit":"USD","group_id":9,"group":{"id":9,"name":"Claude Pro","rate_multiplier":0.42}}`))
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       29,
			Platform: PlatformAnthropic,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-ant-upstream",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 29)
	require.NoError(t, err)
	require.Equal(t, "sub2api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, 6.78, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, "USD", repo.updatedExtra["upstream_balance_unit"])
	require.Equal(t, "Claude Pro", repo.updatedExtra["upstream_group"])
	require.Equal(t, int64(9), repo.updatedExtra["upstream_group_id"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIAdminTokenResolvesEffectiveRate(t *testing.T) {
	billingCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"remaining":9.5,"unit":"USD"}`))
		case "/api/v1/keys":
			require.Equal(t, "Bearer admin-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"items":[{"id":491,"name":"pro","key":"sk-other","group_id":23,"group":{"id":23,"name":"额度模式 - 高可用","rate_multiplier":0.8}},{"id":404,"name":"plus","key":"sk-upstream","group_id":4,"group":{"id":4,"name":"额度模式 - 标准","rate_multiplier":0.4}}]}}`))
		case "/api/v1/groups/rates":
			require.Equal(t, "Bearer admin-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"23":0.17,"4":0.09}}`))
		case "/v1/sub2api/billing":
			billingCalls++
			http.Error(w, "billing fallback should not be called", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       20,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			ChannelPrice: func() *float64 {
				price := 1.23
				return &price
			}(),
			Credentials: map[string]any{
				"base_url":                    srv.URL + "/v1",
				"api_key":                     "sk-upstream",
				"upstream_admin_type":         "sub2api",
				"upstream_admin_access_token": "admin-token",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 20)
	require.NoError(t, err)
	require.Equal(t, "sub2api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, 9.5, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, "额度模式 - 标准", repo.updatedExtra["upstream_group"])
	require.Equal(t, int64(4), repo.updatedExtra["upstream_group_id"])
	require.Equal(t, int64(404), repo.updatedExtra["upstream_key_id"])
	require.Equal(t, 0.4, repo.updatedExtra["upstream_group_rate_multiplier"])
	require.Equal(t, 0.09, repo.updatedExtra["upstream_effective_rate_multiplier"])
	require.Equal(t, "user_group_rate", repo.updatedExtra["upstream_rate_source"])
	require.Zero(t, billingCalls)
	require.NotNil(t, repo.updatedChannelPrice)
	require.Equal(t, 0.09, *repo.updatedChannelPrice)
}

func TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIAdminPasswordLogsInForEffectiveRate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"remaining":3,"unit":"USD"}`))
		case "/api/v1/auth/login":
			require.Equal(t, http.MethodPost, r.Method)
			var payload map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "admin@example.com", payload["email"])
			require.Equal(t, "secret", payload["password"])
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"login-token","token_type":"Bearer"}}`))
		case "/api/v1/keys":
			require.Equal(t, "Bearer login-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"items":[{"id":88,"name":"plus","key":"sk-upstream","group_id":4,"group":{"id":4,"name":"额度模式 - 标准","rate_multiplier":0.4}}]}}`))
		case "/api/v1/groups/rates":
			require.Equal(t, "Bearer login-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"4":0.09}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       21,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                srv.URL + "/v1",
				"api_key":                 "sk-upstream",
				"upstream_admin_type":     "sub2api",
				"upstream_admin_email":    "admin@example.com",
				"upstream_admin_password": "secret",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 21)
	require.NoError(t, err)
	require.Equal(t, "额度模式 - 标准", repo.updatedExtra["upstream_group"])
	require.Equal(t, 0.09, repo.updatedExtra["upstream_effective_rate_multiplier"])
	require.Equal(t, "user_group_rate", repo.updatedExtra["upstream_rate_source"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIAdminRefreshTokenPersistsRotatedTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"remaining":3,"unit":"USD"}`))
		case "/api/v1/auth/refresh":
			require.Equal(t, http.MethodPost, r.Method)
			var payload map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "rt-admin", payload["refresh_token"])
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"refreshed-token","refresh_token":"rt-new","token_type":"Bearer","expires_in":3600}}`))
		case "/api/v1/keys":
			require.Equal(t, "Bearer refreshed-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"items":[{"id":88,"name":"plus","key":"sk-upstream","group_id":4,"group":{"id":4,"name":"额度模式 - 标准","rate_multiplier":0.4}}]}}`))
		case "/api/v1/groups/rates":
			require.Equal(t, "Bearer refreshed-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"4":0.09}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       24,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                     srv.URL + "/v1",
				"api_key":                      "sk-upstream",
				"upstream_admin_type":          "sub2api",
				"upstream_admin_access_token":  "stale-token",
				"upstream_admin_refresh_token": "rt-admin",
				"custom":                       "kept",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 24)
	require.NoError(t, err)
	require.Equal(t, "额度模式 - 标准", repo.updatedExtra["upstream_group"])
	require.Equal(t, "refreshed-token", repo.updatedCredentials["upstream_admin_access_token"])
	require.Equal(t, "rt-new", repo.updatedCredentials["upstream_admin_refresh_token"])
	require.Equal(t, "Bearer", repo.updatedCredentials["upstream_admin_token_type"])
	require.Equal(t, "kept", repo.updatedCredentials["custom"])
	require.Equal(t, "sk-upstream", repo.updatedCredentials["api_key"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIAdminRefreshTokenResolvesEffectiveRate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"remaining":3,"unit":"USD"}`))
		case "/api/v1/auth/login":
			t.Fatalf("login endpoint should not be called when refresh token is configured")
		case "/api/v1/auth/refresh":
			require.Equal(t, http.MethodPost, r.Method)
			var payload map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "rt-admin", payload["refresh_token"])
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"refreshed-token","refresh_token":"rt-new","token_type":"Bearer","expires_in":3600}}`))
		case "/api/v1/keys":
			require.Equal(t, "Bearer refreshed-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"items":[{"id":88,"name":"plus","key":"sk-upstream","group_id":4,"group":{"id":4,"name":"额度模式 - 标准","rate_multiplier":0.4}}]}}`))
		case "/api/v1/groups/rates":
			require.Equal(t, "Bearer refreshed-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"4":0.09}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       23,
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

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 23)
	require.NoError(t, err)
	require.Equal(t, "额度模式 - 标准", repo.updatedExtra["upstream_group"])
	require.Equal(t, int64(4), repo.updatedExtra["upstream_group_id"])
	require.Equal(t, int64(88), repo.updatedExtra["upstream_key_id"])
	require.Equal(t, 0.4, repo.updatedExtra["upstream_group_rate_multiplier"])
	require.Equal(t, 0.09, repo.updatedExtra["upstream_effective_rate_multiplier"])
	require.Equal(t, "user_group_rate", repo.updatedExtra["upstream_rate_source"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIAdminRequiresExplicitProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"remaining":3,"unit":"USD"}`))
		case "/api/v1/keys", "/api/v1/groups/rates":
			t.Fatalf("sub2api admin endpoint should not be called when provider is not explicit: %s", r.URL.Path)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       22,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                    srv.URL + "/v1",
				"api_key":                     "sk-upstream",
				"upstream_admin_access_token": "stale-token",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 22)
	require.NoError(t, err)
	require.NotContains(t, repo.updatedExtra, "upstream_group")
	require.NotContains(t, repo.updatedExtra, "upstream_effective_rate_multiplier")
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPIQuotaMinusUsed(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/v1/usage" {
			http.NotFound(w, r)
			return
		}
		require.Equal(t, "/api/usage/token/", r.URL.Path)
		_, _ = w.Write([]byte(`{"success":true,"data":{"quota":500000,"used_quota":125000}}`))
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       10,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, "new-api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, 375000.0, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, "quota", repo.updatedExtra["upstream_balance_unit"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPIAvailableQuota(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/v1/usage" {
			http.NotFound(w, r)
			return
		}
		require.Equal(t, "/api/usage/token/", r.URL.Path)
		_, _ = w.Write([]byte(`{"success":true,"data":{"available_quota":59.64,"used_quota":17.89,"unit":"USD"}}`))
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       15,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 15)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, "new-api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, 59.64, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, "USD", repo.updatedExtra["upstream_balance_unit"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPITotalAvailable(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/v1/usage" {
			http.NotFound(w, r)
			return
		}
		require.Equal(t, "/api/usage/token/", r.URL.Path)
		_, _ = w.Write([]byte(`{"code":true,"message":"ok","data":{"object":"token_usage","name":"demo","total_granted":0,"total_used":0,"total_available":0,"unlimited_quota":false}}`))
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       16,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 16)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, "new-api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, 0.0, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, "quota", repo.updatedExtra["upstream_balance_unit"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPIUserSelfQuota(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/v1/usage" {
			http.NotFound(w, r)
			return
		}
		switch r.URL.Path {
		case "/api/user/self":
			require.Equal(t, "user-access-token", r.Header.Get("Authorization"))
			require.Equal(t, "738", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":738,"group":"vip","quota":4557913,"used_quota":990499351,"request_count":9777}}`))
		case "/api/token/search":
			require.Equal(t, "user-access-token", r.Header.Get("Authorization"))
			require.Equal(t, "738", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       18,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                  srv.URL + "/v1",
				"api_key":                   "sk-upstream",
				"new_api_user_id":           "738",
				"new_api_user_access_token": "user-access-token",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 18)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, "new-api", repo.updatedExtra["upstream_balance_provider"])
	require.InDelta(t, 9.115826, repo.updatedExtra["upstream_balance_remaining"], 0.000001)
	require.Equal(t, "USD", repo.updatedExtra["upstream_balance_unit"])
	require.Equal(t, "vip", repo.updatedExtra["upstream_group"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPIUserSelfResolvesGroupRate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self":
			require.Equal(t, "user-access-token", r.Header.Get("Authorization"))
			require.Equal(t, "935", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":935,"group":"duijie","quota":49990794,"used_quota":9206}}`))
		case "/api/token/search":
			require.Equal(t, "user-access-token", r.Header.Get("Authorization"))
			require.Equal(t, "935", r.Header.Get("New-Api-User"))
			require.Equal(t, "upstream", r.URL.Query().Get("token"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"page":1,"page_size":10,"total":1,"items":[{"id":976,"user_id":935,"key":"sk-upstream","name":"正价pro","group":"Codex"}]}}`))
		case "/api/user/self/groups":
			require.Equal(t, "new-api-session=abc", r.Header.Get("Cookie"))
			require.Equal(t, "935", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"Codex":{"desc":"Codex分组-0.18/刀","ratio":0.18},"default":{"desc":"默认分组-0.23/刀","ratio":0.23}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       23,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			ChannelPrice: func() *float64 {
				price := 1.23
				return &price
			}(),
			Credentials: map[string]any{
				"base_url":                  srv.URL + "/v1",
				"api_key":                   "sk-upstream",
				"new_api_user_id":           "935",
				"new_api_user_access_token": "user-access-token",
				"new_api_session_cookie":    "new-api-session=abc",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 23)
	require.NoError(t, err)
	require.Equal(t, "new-api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, "Codex", repo.updatedExtra["upstream_group"])
	require.Equal(t, int64(976), repo.updatedExtra["upstream_key_id"])
	require.Equal(t, 0.18, repo.updatedExtra["upstream_effective_rate_multiplier"])
	require.Equal(t, "user_group_rate", repo.updatedExtra["upstream_rate_source"])
	require.NotNil(t, repo.updatedChannelPrice)
	require.Equal(t, 0.18, *repo.updatedChannelPrice)
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPIUserSelfResolvesGroupRateWithAccessTokenOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":114,"group":"default","quota":50000000}}`))
		case "/api/token/search":
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"id":80,"key":"abcd**********mnop","group":"GPT Pro 优惠分组"}]}}`))
		case "/api/user/self/groups":
			require.Equal(t, "user-access-token", r.Header.Get("Authorization"))
			require.Equal(t, "114", r.Header.Get("New-Api-User"))
			require.Empty(t, r.Header.Get("Cookie"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"GPT Pro 优惠分组":{"desc":"优惠分组","ratio":0.08}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{account: &Account{
		ID: 114, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":                  srv.URL,
			"api_key":                   "sk-abcdefghijklmnop",
			"new_api_user_id":           "114",
			"new_api_user_access_token": "user-access-token",
		},
	}}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 114)
	require.NoError(t, err)
	require.Equal(t, "GPT Pro 优惠分组", repo.updatedExtra["upstream_group"])
	require.Equal(t, 0.08, repo.updatedExtra["upstream_effective_rate_multiplier"])
	require.NotNil(t, repo.updatedChannelPrice)
	require.Equal(t, 0.08, *repo.updatedChannelPrice)
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPIUserSelfLogsInForGroupRateWhenCookieMissing(t *testing.T) {
	loginCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self":
			require.Equal(t, "user-access-token", r.Header.Get("Authorization"))
			require.Equal(t, "935", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":935,"group":"duijie","quota":49990794,"used_quota":9206}}`))
		case "/api/token/search":
			require.Equal(t, "user-access-token", r.Header.Get("Authorization"))
			require.Equal(t, "935", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"id":976,"key":"sk-upstream","name":"正价pro","group":"Codex"}]}}`))
		case "/api/user/login":
			loginCalls++
			require.Equal(t, http.MethodPost, r.Method)
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "owner@example.com", body["username"])
			require.Equal(t, "login-secret", body["password"])
			http.SetCookie(w, &http.Cookie{Name: "new-api-session", Value: "auto"})
			_, _ = w.Write([]byte(`{"success":true}`))
		case "/api/user/self/groups":
			if r.Header.Get("Cookie") == "" {
				http.Error(w, "session required", http.StatusUnauthorized)
				return
			}
			require.Equal(t, "new-api-session=auto", r.Header.Get("Cookie"))
			require.Equal(t, "935", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"Codex":{"desc":"Codex分组-0.18/刀","ratio":0.18}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       25,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                  srv.URL + "/v1",
				"api_key":                   "sk-upstream",
				"new_api_user_id":           "935",
				"new_api_user_access_token": "user-access-token",
				"new_api_login_username":    "owner@example.com",
				"new_api_login_password":    "login-secret",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 25)
	require.NoError(t, err)
	require.Equal(t, 1, loginCalls)
	require.Equal(t, "Codex", repo.updatedExtra["upstream_group"])
	require.Equal(t, 0.18, repo.updatedExtra["upstream_effective_rate_multiplier"])
	require.NotNil(t, repo.updatedChannelPrice)
	require.Equal(t, 0.18, *repo.updatedChannelPrice)
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPIUserSelfDoesNotUsePricingDefaultAsEffectiveRate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":935,"group":"duijie","quota":49990794,"used_quota":9206}}`))
		case "/api/token/search":
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"id":976,"key":"sk-upstream","name":"正价pro","group":"Codex"}]}}`))
		case "/api/pricing":
			t.Fatalf("pricing default group ratio must not be used as new-api effective rate")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       24,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":                  srv.URL + "/v1",
				"api_key":                   "sk-upstream",
				"new_api_user_id":           "935",
				"new_api_user_access_token": "user-access-token",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 24)
	require.NoError(t, err)
	require.Equal(t, "new-api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, "Codex", repo.updatedExtra["upstream_group"])
	require.NotContains(t, repo.updatedExtra, "upstream_effective_rate_multiplier")
	require.Nil(t, repo.updatedChannelPrice)
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPINegativeTotalAvailableClampsToZero(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/v1/usage" {
			http.NotFound(w, r)
			return
		}
		require.Equal(t, "/api/usage/token/", r.URL.Path)
		_, _ = w.Write([]byte(`{"code":true,"message":"ok","data":{"object":"token_usage","name":"demo","total_granted":0,"total_used":96557499,"total_available":-96557499,"unlimited_quota":false}}`))
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       17,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	_, err := svc.Refresh(context.Background(), 17)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, "new-api", repo.updatedExtra["upstream_balance_provider"])
	require.Equal(t, 0.0, repo.updatedExtra["upstream_balance_remaining"])
	require.Equal(t, "quota", repo.updatedExtra["upstream_balance_unit"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPIMissingUsedQuotaPersistsErrorSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/usage" {
			http.NotFound(w, r)
			return
		}
		require.Equal(t, "/api/usage/token/", r.URL.Path)
		_, _ = w.Write([]byte(`{"success":true,"data":{"quota":500000}}`))
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       13,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
			Extra: map[string]any{
				"upstream_balance_provider":  "sub2api",
				"upstream_balance_remaining": 12.34,
				"upstream_balance_unit":      "USD",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	account, err := svc.Refresh(context.Background(), 13)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "error", repo.updatedExtra["upstream_balance_status"])
	require.NotContains(t, repo.updatedExtra, "upstream_balance_provider")
	require.NotContains(t, repo.updatedExtra, "upstream_balance_remaining")
	require.NotContains(t, repo.updatedExtra, "upstream_balance_unit")
	require.NotEmpty(t, repo.updatedExtra["upstream_balance_error"])
	require.NotEmpty(t, repo.updatedExtra["upstream_balance_updated_at"])
	require.Equal(t, "sub2api", account.Extra["upstream_balance_provider"])
	require.Equal(t, 12.34, account.Extra["upstream_balance_remaining"])
	require.Equal(t, "USD", account.Extra["upstream_balance_unit"])
	require.Equal(t, "error", account.Extra["upstream_balance_status"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_NewAPIInvalidUsedQuotaPersistsErrorSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/usage" {
			http.NotFound(w, r)
			return
		}
		require.Equal(t, "/api/usage/token/", r.URL.Path)
		_, _ = w.Write([]byte(`{"success":true,"data":{"quota":500000,"used_quota":"oops"}}`))
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       14,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
			Extra: map[string]any{
				"upstream_balance_provider":  "sub2api",
				"upstream_balance_remaining": 12.34,
				"upstream_balance_unit":      "USD",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	account, err := svc.Refresh(context.Background(), 14)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "error", repo.updatedExtra["upstream_balance_status"])
	require.NotContains(t, repo.updatedExtra, "upstream_balance_provider")
	require.NotContains(t, repo.updatedExtra, "upstream_balance_remaining")
	require.NotContains(t, repo.updatedExtra, "upstream_balance_unit")
	require.NotEmpty(t, repo.updatedExtra["upstream_balance_error"])
	require.NotEmpty(t, repo.updatedExtra["upstream_balance_updated_at"])
	require.Equal(t, "sub2api", account.Extra["upstream_balance_provider"])
	require.Equal(t, 12.34, account.Extra["upstream_balance_remaining"])
	require.Equal(t, "USD", account.Extra["upstream_balance_unit"])
	require.Equal(t, "error", account.Extra["upstream_balance_status"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_PersistsErrorSnapshotWhenProvidersFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{
			ID:       12,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url": srv.URL + "/v1",
				"api_key":  "sk-upstream",
			},
			Extra: map[string]any{
				"upstream_balance_provider":  "sub2api",
				"upstream_balance_remaining": 12.34,
				"upstream_balance_unit":      "USD",
			},
		},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, srv.Client())
	account, err := svc.Refresh(context.Background(), 12)
	require.NoError(t, err)
	require.Equal(t, "error", repo.updatedExtra["upstream_balance_status"])
	require.NotEmpty(t, repo.updatedExtra["upstream_balance_error"])
	require.NotContains(t, repo.updatedExtra, "upstream_balance_provider")
	require.NotContains(t, repo.updatedExtra, "upstream_balance_remaining")
	require.NotContains(t, repo.updatedExtra, "upstream_balance_unit")
	require.NotEmpty(t, repo.updatedExtra["upstream_balance_updated_at"])
	require.NotNil(t, account)
	require.Equal(t, "sub2api", account.Extra["upstream_balance_provider"])
	require.Equal(t, 12.34, account.Extra["upstream_balance_remaining"])
	require.Equal(t, "USD", account.Extra["upstream_balance_unit"])
	require.Equal(t, "error", account.Extra["upstream_balance_status"])
}

func TestOpenAIUpstreamBalanceServiceRefresh_RejectsUnsupportedAccount(t *testing.T) {
	repo := &openAIUpstreamBalanceRepoStub{
		account: &Account{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
	}

	svc := NewOpenAIUpstreamBalanceService(repo, http.DefaultClient)
	_, err := svc.Refresh(context.Background(), 11)
	require.Error(t, err)
}

func TestBuildOpenAIUpstreamBalanceUpdates(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	updates := buildOpenAIUpstreamBalanceUpdates(OpenAIUpstreamBalanceSnapshot{
		Provider:  "new-api",
		Remaining: 42.5,
		Unit:      "quota",
		Status:    "ok",
		Error:     "",
		UpdatedAt: now,
		Group:     "vip",
	})

	require.Equal(t, "new-api", updates["upstream_balance_provider"])
	require.Equal(t, 42.5, updates["upstream_balance_remaining"])
	require.Equal(t, "quota", updates["upstream_balance_unit"])
	require.Equal(t, "ok", updates["upstream_balance_status"])
	require.Equal(t, "", updates["upstream_balance_error"])
	require.Equal(t, now.Format(time.RFC3339), updates["upstream_balance_updated_at"])
	require.Equal(t, "vip", updates["upstream_group"])
}
