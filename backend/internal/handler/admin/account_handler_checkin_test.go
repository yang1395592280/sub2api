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

type sub2APICheckinAccountRepoStub struct {
	service.AccountRepository
	account          *service.Account
	updateExtraCalls int
}

func (r *sub2APICheckinAccountRepoStub) GetByID(context.Context, int64) (*service.Account, error) {
	if r.account == nil {
		return nil, service.ErrAccountNotFound
	}
	return r.account, nil
}

func (r *sub2APICheckinAccountRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updateExtraCalls++
	if r.account.Extra == nil {
		r.account.Extra = map[string]any{}
	}
	for k, v := range updates {
		r.account.Extra[k] = v
	}
	return nil
}

type checkinRoundTripFunc func(*http.Request) (*http.Response, error)

func (f checkinRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func checkinRoundTripClient(fn checkinRoundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func checkinJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestAccountHandler_TestUpstreamCheckinReturnsUpdatedAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	loc := time.FixedZone("CST", 8*3600)
	account := &service.Account{
		ID:       9,
		Name:     "sub2api-checkin",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"base_url":                    "https://up.example/v1",
			"api_key":                     "sk-upstream",
			"upstream_admin_type":         "sub2api",
			"upstream_admin_access_token": "access-token",
		},
	}
	checkinSvc := service.NewSub2APICheckinService(
		&sub2APICheckinAccountRepoStub{account: account},
		service.NewOpenAIUpstreamBalanceService(nil, checkinRoundTripClient(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://up.example/api/v1/user/checkin", req.URL.String())
			require.Equal(t, "Bearer access-token", req.Header.Get("Authorization"))
			return checkinJSONResponse(http.StatusOK, `{"code":0,"message":"success","data":{"checked_in":true,"reward_amount":3.5,"balance":88.8}}`), nil
		})),
		loc,
	)

	h := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, checkinSvc)
	r := gin.New()
	r.POST("/accounts/:id/upstream-checkin/test", h.TestUpstreamCheckin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/accounts/9/upstream-checkin/test", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"upstream_checkin_status":"success"`)
	require.Contains(t, w.Body.String(), `"upstream_checkin_reward_amount":3.5`)
}

func TestAccountHandler_TestUpstreamCheckinRejectsNonSub2APIAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &sub2APICheckinAccountRepoStub{
		account: &service.Account{
			ID:       10,
			Name:     "new-api-checkin",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeAPIKey,
			Credentials: map[string]any{
				"base_url":             "https://up.example/v1",
				"api_key":              "sk-upstream",
				"upstream_admin_type":  "new-api",
				"upstream_checkin_url": "/api/v1/user/checkin",
			},
		},
	}
	checkinSvc := service.NewSub2APICheckinService(repo, nil, time.FixedZone("CST", 8*3600))

	h := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, checkinSvc)
	r := gin.New()
	r.POST("/accounts/:id/upstream-checkin/test", h.TestUpstreamCheckin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/accounts/10/upstream-checkin/test", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "SUB2API_CHECKIN_INVALID_ACCOUNT")
	require.Equal(t, 0, repo.updateExtraCalls)
	require.Empty(t, repo.account.Extra)
}
