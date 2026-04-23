package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type sizeBetServiceStub struct {
	currentView *service.SizeBetCurrentRoundView
	currentErr  error
	currentUser int64
	currentNow  time.Time

	placeBetReq service.PlaceSizeBetRequest
	placeBet    *service.SizeBet
	placeBetErr error

	historyItems      []service.SizeBetUserHistoryItem
	historyPagination *pagination.PaginationResult
	historyErr        error
	historyUserID     int64
	historyParams     pagination.PaginationParams

	recentRounds []service.SizeBetRound
	recentLimit  int
	recentErr    error

	leaderboardScope string
	leaderboardNow   time.Time
	leaderboard      *service.SizeBetLeaderboardView
	leaderboardErr   error

	rules    *service.SizeBetRulesView
	rulesNow time.Time
	rulesErr error
}

func (s *sizeBetServiceStub) GetCurrentRoundView(_ context.Context, userID int64, now time.Time) (*service.SizeBetCurrentRoundView, error) {
	s.currentUser = userID
	s.currentNow = now
	return s.currentView, s.currentErr
}

func (s *sizeBetServiceStub) PlaceBet(_ context.Context, req service.PlaceSizeBetRequest) (*service.SizeBet, error) {
	s.placeBetReq = req
	return s.placeBet, s.placeBetErr
}

func (s *sizeBetServiceStub) GetHistory(_ context.Context, userID int64, params pagination.PaginationParams) ([]service.SizeBetUserHistoryItem, *pagination.PaginationResult, error) {
	s.historyUserID = userID
	s.historyParams = params
	return s.historyItems, s.historyPagination, s.historyErr
}

func (s *sizeBetServiceStub) ListRecentRounds(_ context.Context, limit int) ([]service.SizeBetRound, error) {
	s.recentLimit = limit
	return s.recentRounds, s.recentErr
}

func (s *sizeBetServiceStub) GetLeaderboard(_ context.Context, scope string, now time.Time) (*service.SizeBetLeaderboardView, error) {
	s.leaderboardScope = scope
	s.leaderboardNow = now
	return s.leaderboard, s.leaderboardErr
}

func (s *sizeBetServiceStub) GetRules(_ context.Context, now time.Time) (*service.SizeBetRulesView, error) {
	s.rulesNow = now
	return s.rules, s.rulesErr
}

func decodeEnvelope(t *testing.T, w *httptest.ResponseRecorder) response.Response {
	t.Helper()

	var resp response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func intPtr(v int) *int {
	return &v
}

func TestSizeBetHandlerPlaceBetReturnsConflictAfterClose(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &sizeBetServiceStub{placeBetErr: service.ErrSizeBetClosed}
	h := &SizeBetHandler{service: svc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/game/size-bet/bet", strings.NewReader(`{
		"round_id": 1001,
		"direction": "small",
		"stake_amount": 5,
		"idempotency_key": "bet-1001-9"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PlaceBet(c)

	require.Equal(t, http.StatusConflict, w.Code)
	require.Equal(t, int64(9), svc.placeBetReq.UserID)
	require.Equal(t, int64(1001), svc.placeBetReq.RoundID)
	require.Equal(t, service.SizeBetDirectionSmall, svc.placeBetReq.Direction)
}

func TestSizeBetHandlerGetCurrentReturnsViewForAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &sizeBetServiceStub{
		currentView: &service.SizeBetCurrentRoundView{
			Enabled: true,
			Phase:   service.SizeBetPhaseBetting,
			Round: &service.SizeBetCurrentRound{
				ID:               11,
				RoundNo:          1001,
				ServerSeedHash:   "seed-hash",
				CountdownSeconds: 18,
			},
			MyBet: &service.SizeBet{
				ID:             21,
				RoundID:        11,
				UserID:         9,
				Direction:      service.SizeBetDirectionBig,
				StakeAmount:    10,
				Status:         service.SizeBetStatusPlaced,
				IdempotencyKey: "internal-key",
			},
			PreviousRound: &service.SizeBetRound{
				ID:              10,
				GameKey:         service.SizeBetGameKey,
				RoundNo:         1000,
				Status:          service.SizeBetRoundStatusSettled,
				ServerSeedHash:  "prev-hash",
				ServerSeed:      "revealed-seed",
				ResultNumber:    intPtr(6),
				ResultDirection: service.SizeBetDirectionMid,
			},
		},
	}
	h := &SizeBetHandler{service: svc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/game/size-bet/current", nil)

	h.GetCurrent(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(9), svc.currentUser)
	require.False(t, svc.currentNow.IsZero())

	resp := decodeEnvelope(t, w)
	data := resp.Data.(map[string]any)
	require.Equal(t, "betting", data["phase"])
	require.Equal(t, true, data["enabled"])
	round := data["round"].(map[string]any)
	require.Contains(t, round, "round_no")
	require.NotContains(t, round, "RoundNo")
	myBet := data["my_bet"].(map[string]any)
	require.Contains(t, myBet, "round_id")
	require.NotContains(t, myBet, "RoundID")
	require.NotContains(t, myBet, "user_id")
	require.NotContains(t, myBet, "idempotency_key")
	previousRound := data["previous_round"].(map[string]any)
	require.Contains(t, previousRound, "round_no")
	require.NotContains(t, previousRound, "GameKey")
}

func TestSizeBetHandlerGetHistoryReturnsPaginatedData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &sizeBetServiceStub{
		historyItems: []service.SizeBetUserHistoryItem{
			{BetID: 3, RoundNo: 1001, Status: service.SizeBetStatusWon},
		},
		historyPagination: &pagination.PaginationResult{
			Total:    1,
			Page:     2,
			PageSize: 5,
			Pages:    1,
		},
	}
	h := &SizeBetHandler{service: svc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/game/size-bet/history?page=2&page_size=5", nil)

	h.GetHistory(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(9), svc.historyUserID)
	require.Equal(t, 2, svc.historyParams.Page)
	require.Equal(t, 5, svc.historyParams.PageSize)

	resp := decodeEnvelope(t, w)
	data := resp.Data.(map[string]any)
	require.Equal(t, float64(1), data["total"])
}

func TestSizeBetHandlerPlaceBetUsesDTOWithoutInternalFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &sizeBetServiceStub{
		placeBet: &service.SizeBet{
			ID:             55,
			RoundID:        1001,
			UserID:         9,
			Direction:      service.SizeBetDirectionSmall,
			StakeAmount:    5,
			Status:         service.SizeBetStatusPlaced,
			IdempotencyKey: "internal-key",
			PlacedAt:       time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC),
		},
	}
	h := &SizeBetHandler{service: svc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/game/size-bet/bet", strings.NewReader(`{
		"round_id": 1001,
		"direction": "small",
		"stake_amount": 5,
		"idempotency_key": "bet-1001-9"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PlaceBet(c)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeEnvelope(t, w)
	data := resp.Data.(map[string]any)
	require.Contains(t, data, "round_id")
	require.NotContains(t, data, "RoundID")
	require.NotContains(t, data, "user_id")
	require.NotContains(t, data, "idempotency_key")
}

func TestSizeBetHandlerListRecentRoundsUsesDTO(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &sizeBetServiceStub{
		recentRounds: []service.SizeBetRound{
			{
				ID:              10,
				GameKey:         service.SizeBetGameKey,
				RoundNo:         1000,
				Status:          service.SizeBetRoundStatusSettled,
				ServerSeedHash:  "hash-0",
				ServerSeed:      "seed-0",
				ResultNumber:    intPtr(6),
				ResultDirection: service.SizeBetDirectionMid,
			},
		},
	}
	h := &SizeBetHandler{service: svc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/game/size-bet/rounds?limit=5", nil)

	h.ListRecentRounds(c)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeEnvelope(t, w)
	items := resp.Data.([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	require.Contains(t, item, "round_no")
	require.NotContains(t, item, "GameKey")
	require.NotContains(t, item, "RoundNo")
}
