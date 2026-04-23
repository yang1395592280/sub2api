package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserHandlerGetBalanceHistoryReturnsUnifiedTimeline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	adminSvc.timelineItems = []service.UserActivityTimelineItem{
		{
			ID:        "game-1",
			Type:      "game_net",
			Summary:   "Size Bet #1001",
			Value:     10,
			CreatedAt: time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC),
			Details: map[string]any{
				"round_no":      1001,
				"stake_amount":  10,
				"payout_amount": 20,
			},
		},
		{
			ID:        "checkin-2",
			Type:      "checkin_reward",
			Summary:   "Daily check-in reward",
			Value:     0.02,
			CreatedAt: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC),
		},
	}

	router := gin.New()
	router.GET("/api/v1/admin/users/:id/balance-history", NewUserHandler(adminSvc, nil).GetBalanceHistory)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/1/balance-history?page=1&page_size=20", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ID      string         `json:"id"`
				Type    string         `json:"type"`
				Summary string         `json:"summary"`
				Details map[string]any `json:"details"`
			} `json:"items"`
			Total          int64   `json:"total"`
			TotalRecharged float64 `json:"total_recharged"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Len(t, payload.Data.Items, 2)
	require.Equal(t, "game_net", payload.Data.Items[0].Type)
	require.Equal(t, "checkin_reward", payload.Data.Items[1].Type)
	require.EqualValues(t, 1001, payload.Data.Items[0].Details["round_no"])
	require.Equal(t, int64(2), payload.Data.Total)
	require.Equal(t, 100.0, payload.Data.TotalRecharged)
}
