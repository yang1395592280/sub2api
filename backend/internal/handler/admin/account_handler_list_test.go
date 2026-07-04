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

type stubOpenAIAutoSchedulerAccountSummaryService struct {
	summaries  map[int64]service.OpenAIAutoSchedulerAccountSummary
	groupID    int64
	accountIDs []int64
}

func (s *stubOpenAIAutoSchedulerAccountSummaryService) ListAccountSummaries(_ context.Context, groupID int64, accountIDs []int64) (map[int64]service.OpenAIAutoSchedulerAccountSummary, error) {
	s.groupID = groupID
	s.accountIDs = append([]int64(nil), accountIDs...)
	return s.summaries, nil
}

func setupAccountListRouter() (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
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

func TestAccountHandlerListIncludesOpenAIAutoSchedulerSummaryForGroup(t *testing.T) {
	router, adminSvc := setupAccountListRouter()
	now := time.Now().UTC()
	adminSvc.accounts = []service.Account{
		{ID: 7, Name: "openai-account", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	speed := 220
	summarySvc := &stubOpenAIAutoSchedulerAccountSummaryService{
		summaries: map[int64]service.OpenAIAutoSchedulerAccountSummary{
			7: {
				State:         service.OpenAIAutoSchedulerStateRunning,
				SpeedPriority: 1,
				SpeedMS:       &speed,
				ProbeModel:    "gpt-5.5",
			},
		},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler.SetOpenAIAutoSchedulerAccountSummaryService(summarySvc)
	router = gin.New()
	router.GET("/api/v1/admin/accounts", handler.List)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20&group=10", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(10), summarySvc.groupID)
	require.Equal(t, []int64{7}, summarySvc.accountIDs)
	require.Contains(t, rec.Body.String(), `"openai_auto_scheduler"`)
	require.Contains(t, rec.Body.String(), `"speed_priority":1`)
	require.Contains(t, rec.Body.String(), `"probe_model":"gpt-5.5"`)
}

func TestAccountHandlerListUsesFirstAccountGroupForOpenAIAutoSchedulerSummary(t *testing.T) {
	router, adminSvc := setupAccountListRouter()
	now := time.Now().UTC()
	adminSvc.accounts = []service.Account{
		{ID: 7, Name: "openai-account", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, GroupIDs: []int64{10}, CreatedAt: now, UpdatedAt: now},
	}
	speed := 220
	summarySvc := &stubOpenAIAutoSchedulerAccountSummaryService{
		summaries: map[int64]service.OpenAIAutoSchedulerAccountSummary{
			7: {
				State:         service.OpenAIAutoSchedulerStateRunning,
				SpeedPriority: 1,
				SpeedMS:       &speed,
				ProbeModel:    "gpt-5.5",
			},
		},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler.SetOpenAIAutoSchedulerAccountSummaryService(summarySvc)
	router = gin.New()
	router.GET("/api/v1/admin/accounts", handler.List)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(10), summarySvc.groupID)
	require.Equal(t, []int64{7}, summarySvc.accountIDs)
	require.Contains(t, rec.Body.String(), `"openai_auto_scheduler"`)
}
