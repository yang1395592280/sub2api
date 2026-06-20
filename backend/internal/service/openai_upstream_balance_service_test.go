package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAIUpstreamBalanceRepoStub struct {
	AccountRepository
	account      *Account
	updatedExtra map[string]any
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

func TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIUsageRemaining(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/usage", r.URL.Path)
		require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"remaining":12.34,"unit":"USD","balance":99}`))
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

func TestOpenAIUpstreamBalanceServiceRefresh_RejectsNonOpenAIAPIKey(t *testing.T) {
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
	})

	require.Equal(t, "new-api", updates["upstream_balance_provider"])
	require.Equal(t, 42.5, updates["upstream_balance_remaining"])
	require.Equal(t, "quota", updates["upstream_balance_unit"])
	require.Equal(t, "ok", updates["upstream_balance_status"])
	require.Equal(t, "", updates["upstream_balance_error"])
	require.Equal(t, now.Format(time.RFC3339), updates["upstream_balance_updated_at"])
}
