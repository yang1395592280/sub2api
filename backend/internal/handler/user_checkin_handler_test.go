//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type checkinHandlerRepository struct {
	service.CheckinRepository
	service.CheckinRiskRepository
	invalid bool
	events  []string
	credits int
}

func (r *checkinHandlerRepository) RecordCheckinSecurityEvent(_ context.Context, _ int64, event, _, _ string) error {
	r.events = append(r.events, event)
	return nil
}

func (r *checkinHandlerRepository) ConsumeCheckinPageToken(context.Context, int64, string) (bool, error) {
	return !r.invalid, nil
}

func (r *checkinHandlerRepository) HasCheckedInOnDate(context.Context, int64, string) (bool, error) {
	return false, nil
}

func (r *checkinHandlerRepository) CreateAndCredit(_ context.Context, record *service.CheckinRecord) (*service.CheckinRecord, error) {
	r.credits++
	return record, nil
}

func TestCheckinHandlerCaptchaAndEvidence(t *testing.T) {
	for _, tc := range []struct {
		name          string
		authenticated bool
		required      bool
		configured    bool
		body          string
		status        int
		events        []string
		credits       int
		invalid       bool
	}{
		{name: "authentication required", body: `{}`, status: http.StatusUnauthorized},
		{name: "configuration outage is not evidence", authenticated: true, required: true, body: `{"page_token":"` + strings.Repeat("a", 64) + `"}`, status: http.StatusServiceUnavailable},
		{name: "rejected captcha prevents credit", authenticated: true, configured: true, body: `{"page_token":"` + strings.Repeat("a", 64) + `"}`, status: http.StatusBadRequest, events: []string{"captcha_failure"}},
		{name: "honeypot refuses request", authenticated: true, body: `{"website":"bot"}`, status: http.StatusForbidden, events: []string{"honeypot"}},
		{name: "missing token", authenticated: true, body: `{}`, status: http.StatusForbidden, events: []string{"page_bypass"}},
		{name: "replayed token", authenticated: true, invalid: true, body: `{"page_token":"` + strings.Repeat("a", 64) + `"}`, status: http.StatusForbidden, events: []string{"page_bypass"}},
		{name: "valid token credits", authenticated: true, body: `{"page_token":"` + strings.Repeat("a", 64) + `"}`, status: http.StatusOK, credits: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &checkinHandlerRepository{invalid: tc.invalid}
			values := map[string]string{service.SettingKeyCheckinEnabled: "true", service.SettingKeyCheckinMinReward: "1", service.SettingKeyCheckinMaxReward: "1"}
			cfg := &config.Config{}
			cfg.Server.Mode = "release"
			cfg.Turnstile.Required = tc.required
			var settings *service.SettingService
			var turnstile *service.TurnstileService
			if tc.configured {
				values[service.SettingKeyTurnstileEnabled] = "true"
				values[service.SettingKeyTurnstileSecretKey] = "test-secret"
				settings = service.NewSettingService(&oauthCaptchaSettingRepo{values: values}, cfg)
				turnstile = service.NewTurnstileService(settings, nil)
			}
			auth := service.NewAuthService(nil, nil, nil, nil, cfg, settings, nil, turnstile, nil, nil, nil, nil, nil)
			h := NewCheckinHandler(service.NewCheckinService(repo, &oauthCaptchaSettingRepo{values: values}, nil, nil), auth)
			router := gin.New()
			router.POST("/checkin", func(c *gin.Context) {
				if tc.authenticated {
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
				}
				h.Claim(c)
			})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/checkin", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.Equal(t, tc.events, repo.events)
			require.Equal(t, tc.credits, repo.credits)
		})
	}
}
