package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userActivityTimelineStub struct {
	lastUserID   int64
	lastPage     int
	lastPageSize int
	lastType     string
}

func (s *userActivityTimelineStub) GetUserBalanceHistory(_ context.Context, userID int64, page, pageSize int, codeType string) ([]service.UserActivityTimelineItem, int64, float64, error) {
	s.lastUserID = userID
	s.lastPage = page
	s.lastPageSize = pageSize
	s.lastType = codeType

	return []service.UserActivityTimelineItem{
		{
			ID:        "game-1",
			Type:      "game_net",
			Summary:   "",
			Value:     10,
			CreatedAt: time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC),
			Details: map[string]any{
				"round_no": 1001,
			},
		},
	}, 1, 12.5, nil
}

func TestUserHandlerGetBalanceHistoryDirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	activitySvc := &userActivityTimelineStub{}
	h := NewUserHandler(nil, nil, nil, nil, nil, activitySvc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 9})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/balance-history?page=2&page_size=15&type=game", nil)
	q := c.Request.URL.Query()
	q.Set("page", "2")
	q.Set("page_size", "15")
	q.Set("type", "game")
	c.Request.URL.RawQuery = q.Encode()

	h.GetBalanceHistory(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(9), activitySvc.lastUserID)
	require.Equal(t, 2, activitySvc.lastPage)
	require.Equal(t, 15, activitySvc.lastPageSize)
	require.Equal(t, "game", activitySvc.lastType)

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"items"`
			Total          int64   `json:"total"`
			TotalRecharged float64 `json:"total_recharged"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, "game_net", payload.Data.Items[0].Type)
	require.Equal(t, int64(1), payload.Data.Total)
	require.Equal(t, 12.5, payload.Data.TotalRecharged)
}
