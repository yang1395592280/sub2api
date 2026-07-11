package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubZenxiangLiyuService struct {
	status       *service.ZenxiangLiyuStatus
	statusErr    error
	playResult   *service.ZenxiangLiyuPlayResult
	playErr      error
	lastUserID   int64
	lastRequest  string
	lastRecordID int64
}

func (s *stubZenxiangLiyuService) GetStatus(_ context.Context, userID int64) (*service.ZenxiangLiyuStatus, error) {
	s.lastUserID = userID
	return s.status, s.statusErr
}

func (s *stubZenxiangLiyuService) Play(_ context.Context, userID int64, requestID string) (*service.ZenxiangLiyuPlayResult, error) {
	s.lastUserID = userID
	s.lastRequest = requestID
	return s.playResult, s.playErr
}

func (s *stubZenxiangLiyuService) PlayLuckyCoin(_ context.Context, userID, recordID int64) (*service.ZenxiangLiyuLuckyCoinResult, error) {
	s.lastUserID = userID
	s.lastRecordID = recordID
	return &service.ZenxiangLiyuLuckyCoinResult{RecordID: recordID, Outcome: "double"}, nil
}

func (s *stubZenxiangLiyuService) ListUserRecords(context.Context, int64, int, int) ([]service.ZenxiangLiyuRecord, int, error) {
	return nil, 0, nil
}
func (s *stubZenxiangLiyuService) GetUserDailySummary(context.Context, int64) (*service.ZenxiangLiyuDailySummary, error) {
	return &service.ZenxiangLiyuDailySummary{}, nil
}

func TestZenxiangLiyuHandlerPlayRejectsMissingRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewZenxiangLiyuHandler(&stubZenxiangLiyuService{})
	router := authenticatedZenxiangLiyuTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/zenxiang-liyu/play", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestZenxiangLiyuHandlerStatusReturnsServicePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubZenxiangLiyuService{status: &service.ZenxiangLiyuStatus{Visible: true, CanPlay: true, TicketAmount: 2}}
	h := NewZenxiangLiyuHandler(svc)
	router := authenticatedZenxiangLiyuTestRouter(h)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/zenxiang-liyu/status", nil))

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"visible":true`)
	require.Equal(t, int64(42), svc.lastUserID)
}

func TestZenxiangLiyuHandlerPlayPassesOnlyRequestIDToService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubZenxiangLiyuService{playResult: &service.ZenxiangLiyuPlayResult{RequestID: "request-1"}}
	h := NewZenxiangLiyuHandler(svc)
	router := authenticatedZenxiangLiyuTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/zenxiang-liyu/play", strings.NewReader(`{"request_id":"request-1","reward_amount":999}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(42), svc.lastUserID)
	require.Equal(t, "request-1", svc.lastRequest)
}

func TestZenxiangLiyuHandlerPlayMapsServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		err        error
		statusCode int
	}{
		{name: "request ID required", err: service.ErrZenxiangLiyuRequestIDRequired, statusCode: http.StatusBadRequest},
		{name: "disabled", err: service.ErrZenxiangLiyuDisabled, statusCode: http.StatusForbidden},
		{name: "unauthorized", err: service.ErrZenxiangLiyuUnauthorized, statusCode: http.StatusForbidden},
		{name: "insufficient balance", err: service.ErrZenxiangLiyuInsufficientBalance, statusCode: http.StatusBadRequest},
		{name: "daily limit", err: service.ErrZenxiangLiyuDailyLimitReached, statusCode: http.StatusBadRequest},
		{name: "unexpected", err: errors.New("storage unavailable"), statusCode: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewZenxiangLiyuHandler(&stubZenxiangLiyuService{playErr: tc.err})
			router := authenticatedZenxiangLiyuTestRouter(h)
			req := httptest.NewRequest(http.MethodPost, "/zenxiang-liyu/play", strings.NewReader(`{"request_id":"request-1"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, tc.statusCode, w.Code)
		})
	}
}

func authenticatedZenxiangLiyuTestRouter(h *ZenxiangLiyuHandler) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	})
	router.GET("/zenxiang-liyu/status", h.GetStatus)
	router.POST("/zenxiang-liyu/play", h.Play)
	router.POST("/zenxiang-liyu/records/:id/lucky-coin", h.PlayLuckyCoin)
	router.GET("/zenxiang-liyu/records", h.ListRecords)
	router.GET("/zenxiang-liyu/daily-summary", h.GetDailySummary)
	return router
}
