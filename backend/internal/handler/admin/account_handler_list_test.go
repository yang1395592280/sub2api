package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAccountListRouter() (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts", handler.List)
	return router, adminSvc
}

func TestAccountHandlerListIncludesCreatedAt(t *testing.T) {
	router, adminSvc := setupAccountListRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20&sort_by=created_at&sort_order=desc", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "created_at", adminSvc.lastListAccounts.sortBy)

	var payload struct {
		Data struct {
			Items []struct {
				ID        int64  `json:"id"`
				CreatedAt string `json:"created_at"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)

	createdAt := payload.Data.Items[0].CreatedAt
	require.NotEmpty(t, createdAt)
	require.True(t, strings.HasSuffix(createdAt, "Z"), "created_at should be serialized as UTC")
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	require.NoError(t, err)
	_, offset := parsed.Zone()
	require.Equal(t, 0, offset)
}

type stubOpenAIAccountHealthProvider struct {
	snapshots map[int64]service.OpenAIAccountHealthSnapshot
}

func (s stubOpenAIAccountHealthProvider) SnapshotOpenAIAccountHealth(_ context.Context, accountID int64) (service.OpenAIAccountHealthSnapshot, bool) {
	snapshot, ok := s.snapshots[accountID]
	return snapshot, ok
}

func TestAccountHandlerListUsesOpenAISchedulerHealthSnapshotForStability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{
			ID:          11868,
			Name:        "openai-degraded",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
	}
	healthProvider := stubOpenAIAccountHealthProvider{
		snapshots: map[int64]service.OpenAIAccountHealthSnapshot{
			11868: {
				AccountID:         11868,
				HealthScore:       0,
				Tier:              service.OpenAISchedulerTierDegraded,
				DegradeReason:     "upstream_5xx",
				SuccessRateEWMA:   0,
				ErrorRateEWMA:     1,
				ConsecutiveErrors: 3,
				DecisionReason:    "account is degraded because of upstream_5xx",
			},
		},
	}
	adminSvc.openAIHealthProvider = healthProvider
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts", handler.List)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20&platform=openai", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Data struct {
			Items []struct {
				ID        int64 `json:"id"`
				Stability struct {
					Level  string `json:"level"`
					Label  string `json:"label"`
					Reason string `json:"reason"`
				} `json:"stability"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, int64(11868), payload.Data.Items[0].ID)
	require.Equal(t, "down", payload.Data.Items[0].Stability.Level)
	require.Equal(t, "降级", payload.Data.Items[0].Stability.Label)
	require.Equal(t, "upstream_5xx", payload.Data.Items[0].Stability.Reason)
}
