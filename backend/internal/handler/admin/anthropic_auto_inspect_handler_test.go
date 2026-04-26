package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type anthropicAutoInspectAdminStub struct {
	logs     []service.AnthropicAutoInspectLog
	batches  []service.AnthropicAutoInspectBatch
	settings *service.AnthropicAutoInspectSettings
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

func (s *anthropicAutoInspectAdminStub) ListLogs(context.Context, pagination.PaginationParams, service.AnthropicAutoInspectLogFilter) ([]service.AnthropicAutoInspectLog, *pagination.PaginationResult, error) {
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
