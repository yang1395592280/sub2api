package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type anthropicAutoInspectAdminStub struct {
	logs          []service.AnthropicAutoInspectLog
	batches       []service.AnthropicAutoInspectBatch
	settings      *service.AnthropicAutoInspectSettings
	lastLogFilter service.AnthropicAutoInspectLogFilter
}

func (s *anthropicAutoInspectAdminStub) GetSettings(context.Context) (*service.AnthropicAutoInspectSettings, error) {
	if s.settings == nil {
		return &service.AnthropicAutoInspectSettings{}, nil
	}
	return s.settings, nil
}

func (s *anthropicAutoInspectAdminStub) UpdateSettings(context.Context, service.AnthropicAutoInspectSettings) error {
	return nil
}

func (s *anthropicAutoInspectAdminStub) RunNow(context.Context) error {
	return nil
}

func (s *anthropicAutoInspectAdminStub) ListLogs(_ context.Context, _ pagination.PaginationParams, filter service.AnthropicAutoInspectLogFilter) ([]service.AnthropicAutoInspectLog, *pagination.PaginationResult, error) {
	s.lastLogFilter = filter
	return s.logs, &pagination.PaginationResult{Total: int64(len(s.logs)), Page: 1, PageSize: 20, Pages: 1}, nil
}

func (s *anthropicAutoInspectAdminStub) ListBatches(context.Context, pagination.PaginationParams) ([]service.AnthropicAutoInspectBatch, *pagination.PaginationResult, error) {
	return s.batches, &pagination.PaginationResult{Total: int64(len(s.batches)), Page: 1, PageSize: 20, Pages: 1}, nil
}

func TestAnthropicAutoInspectHandler_ListLogs(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h := &AnthropicAutoInspectHandler{
		svc: &anthropicAutoInspectAdminStub{
			logs: []service.AnthropicAutoInspectLog{
				{ID: 1, AccountID: 9, AccountNameSnapshot: "anthropic-a", Result: service.AnthropicAutoInspectResultError},
			},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/anthropic-auto-inspect/logs?page=1&page_size=20", nil)

	h.ListLogs(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"anthropic-a"`)
	require.Contains(t, w.Body.String(), `"account_name_snapshot":"anthropic-a"`)
	require.NotContains(t, w.Body.String(), `"AccountNameSnapshot"`)
	require.Contains(t, w.Body.String(), `"pagination":{"total":1,"page":1,"page_size":20,"pages":1}`)
}

func TestAnthropicAutoInspectHandler_ListBatches_UsesSnakeCaseJSON(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h := &AnthropicAutoInspectHandler{
		svc: &anthropicAutoInspectAdminStub{
			batches: []service.AnthropicAutoInspectBatch{
				{
					ID:                7,
					TriggerSource:     service.AnthropicAutoInspectTriggerSourceManual,
					Status:            service.AnthropicAutoInspectBatchStatusCompleted,
					TotalAccounts:     3,
					ProcessedAccounts: 3,
					SuccessCount:      2,
					RateLimitedCount:  1,
				},
			},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/anthropic-auto-inspect/batches?page=1&page_size=20", nil)

	h.ListBatches(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"total_accounts":3`)
	require.NotContains(t, w.Body.String(), `"TotalAccounts"`)
}

func TestAnthropicAutoInspectHandler_ListLogs_ParsesTimeRangeQuery(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	stub := &anthropicAutoInspectAdminStub{}
	h := &AnthropicAutoInspectHandler{svc: stub}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/admin/anthropic-auto-inspect/logs?started_from=2026-04-26T12:00:00Z&started_to=2026-04-26T13:00:00Z",
		nil,
	)

	h.ListLogs(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, stub.lastLogFilter.StartedFrom)
	require.NotNil(t, stub.lastLogFilter.StartedTo)
	require.Equal(t, time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC), stub.lastLogFilter.StartedFrom.UTC())
	require.Equal(t, time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC), stub.lastLogFilter.StartedTo.UTC())
}

func TestAnthropicAutoInspectHandler_UpdateSettings(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	body := strings.NewReader(`{"enabled":true,"interval_minutes":1,"error_cooldown_minutes":30}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/admin/anthropic-auto-inspect/settings", body)
	c.Request.Header.Set("Content-Type", "application/json")

	h := &AnthropicAutoInspectHandler{svc: &anthropicAutoInspectAdminStub{}}
	h.UpdateSettings(c)
	require.Equal(t, http.StatusOK, w.Code)
}
