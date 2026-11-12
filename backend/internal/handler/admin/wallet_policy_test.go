//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type walletPolicyRepoStub struct {
	service.SettingRepository
	values map[string]string
}

func (r *walletPolicyRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}
func (r *walletPolicyRepoStub) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func TestWalletPolicySettingsRoundTripAndRejectInvalid(t *testing.T) {
	repo := &walletPolicyRepoStub{values: map[string]string{}}
	h := NewCheckinHandler(service.NewCheckinService(nil, repo, nil, nil))
	router := gin.New()
	router.GET("/policy", h.WalletPolicy)
	router.PUT("/policy", h.WalletPolicy)
	call := func(method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/policy", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	require.JSONEq(t, `{"code":0,"message":"success","data":{"paid_units":2,"welfare_units":1}}`, call("GET", "").Body.String())
	for _, body := range []string{`{"paid_units":3,"welfare_units":1}`, `{"paid_units":3,"welfare_units":0}`} {
		w := call("PUT", body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.JSONEq(t, body, repo.values[service.SettingKeyWalletUsagePolicy])
		require.Contains(t, call("GET", "").Body.String(), `"paid_units":3`)
	}
	before := repo.values[service.SettingKeyWalletUsagePolicy]
	for _, body := range []string{`{}`, `{"paid_units":0,"welfare_units":1}`, `{"paid_units":2,"welfare_units":-1}`, `{"paid_units":1.1,"welfare_units":1}`, `{"paid_units":2,"welfare_units":10001}`} {
		w := call("PUT", body)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		require.Equal(t, before, repo.values[service.SettingKeyWalletUsagePolicy])
	}
}
