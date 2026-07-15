package admin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type upstreamBalanceAccountRepoStub struct {
	service.AccountRepository
	account *service.Account
	updates map[string]any
}

func (r *upstreamBalanceAccountRepoStub) GetByID(context.Context, int64) (*service.Account, error) {
	if r.account == nil {
		return nil, service.ErrAccountNotFound
	}
	return r.account, nil
}

func (r *upstreamBalanceAccountRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.account.Extra == nil {
		r.account.Extra = map[string]any{}
	}
	for k, v := range updates {
		r.account.Extra[k] = v
	}
	r.updates = updates
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func roundTripClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestAccountHandlerRefreshUpstreamBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := &service.Account{
		ID:       7,
		Name:     "openai-upstream",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"base_url": "https://up.example/v1",
			"api_key":  "sk",
		},
	}
	balanceSvc := service.NewOpenAIUpstreamBalanceService(&upstreamBalanceAccountRepoStub{
		account: account,
		updates: map[string]any{
			"upstream_balance_provider":   "sub2api",
			"upstream_balance_remaining":  1.25,
			"upstream_balance_unit":       "USD",
			"upstream_balance_status":     "ok",
			"upstream_balance_updated_at": time.Now().UTC().Format(time.RFC3339),
		},
	}, roundTripClient(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "https://up.example/v1/usage", req.URL.String())
		return jsonResponse(http.StatusOK, `{"remaining":1.25,"unit":"USD"}`), nil
	}))

	h := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, balanceSvc, nil)
	r := gin.New()
	r.POST("/accounts/:id/upstream-balance/refresh", h.RefreshUpstreamBalance)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/accounts/7/upstream-balance/refresh", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"upstream_balance_remaining":1.25`)
}
