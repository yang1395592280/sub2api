package admin

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

type sizeBetSettingsServiceStub struct {
	settings    *service.SizeBetSettings
	settingsErr error

	updateReq service.UpdateSizeBetSettingsRequest
	updateErr error
}

func (s *sizeBetSettingsServiceStub) GetSettings(context.Context) (*service.SizeBetSettings, error) {
	return s.settings, s.settingsErr
}

func (s *sizeBetSettingsServiceStub) UpdateSettings(_ context.Context, req service.UpdateSizeBetSettingsRequest) error {
	s.updateReq = req
	return s.updateErr
}

type sizeBetAdminGameServiceStub struct {
	rounds           []service.SizeBetRound
	roundsPagination *pagination.PaginationResult
	roundsErr        error
	roundsParams     pagination.PaginationParams

	bets           []service.SizeBetAdminBet
	betsPagination *pagination.PaginationResult
	betsErr        error
	betsParams     pagination.PaginationParams
	betsFilter     service.SizeBetAdminBetFilter

	ledger           []service.SizeBetLedgerEntry
	ledgerPagination *pagination.PaginationResult
	ledgerErr        error
	ledgerParams     pagination.PaginationParams
	ledgerFilter     service.SizeBetAdminLedgerFilter

	refundResult *service.SizeBetRefundResult
	refundErr    error
	refundRound  int64
	refundAt     time.Time
}

func (s *sizeBetAdminGameServiceStub) ListRounds(_ context.Context, params pagination.PaginationParams) ([]service.SizeBetRound, *pagination.PaginationResult, error) {
	s.roundsParams = params
	return s.rounds, s.roundsPagination, s.roundsErr
}

func (s *sizeBetAdminGameServiceStub) ListBets(_ context.Context, params pagination.PaginationParams, filter service.SizeBetAdminBetFilter) ([]service.SizeBetAdminBet, *pagination.PaginationResult, error) {
	s.betsParams = params
	s.betsFilter = filter
	return s.bets, s.betsPagination, s.betsErr
}

func (s *sizeBetAdminGameServiceStub) ListLedger(_ context.Context, params pagination.PaginationParams, filter service.SizeBetAdminLedgerFilter) ([]service.SizeBetLedgerEntry, *pagination.PaginationResult, error) {
	s.ledgerParams = params
	s.ledgerFilter = filter
	return s.ledger, s.ledgerPagination, s.ledgerErr
}

func (s *sizeBetAdminGameServiceStub) RefundRound(_ context.Context, roundID int64, refundedAt time.Time) (*service.SizeBetRefundResult, error) {
	s.refundRound = roundID
	s.refundAt = refundedAt
	return s.refundResult, s.refundErr
}

func decodeAdminEnvelope(t *testing.T, w *httptest.ResponseRecorder) response.Response {
	t.Helper()

	var resp response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func TestAdminSizeBetHandlerUpdateSettingsPersistsPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingsSvc := &sizeBetSettingsServiceStub{}
	gameSvc := &sizeBetAdminGameServiceStub{}
	h := &SizeBetHandler{settingsService: settingsSvc, gameService: gameSvc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/games/size-bet/settings", strings.NewReader(`{
		"enabled": true,
		"round_duration_seconds": 60,
		"bet_close_offset_seconds": 50,
		"allowed_stakes": [2, 5, 10],
		"probabilities": {"small": 45, "mid": 10, "big": 45},
		"odds": {"small": 2, "mid": 10, "big": 2},
		"rules_markdown": "rules"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateSettings(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 60, settingsSvc.updateReq.RoundDurationSeconds)
	require.Equal(t, []int{2, 5, 10}, settingsSvc.updateReq.AllowedStakes)
}

func TestAdminSizeBetHandlerRefundRoundReturnsSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingsSvc := &sizeBetSettingsServiceStub{}
	gameSvc := &sizeBetAdminGameServiceStub{
		refundResult: &service.SizeBetRefundResult{
			RoundID:       1001,
			RefundedCount: 2,
			RefundedAt:    time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC),
		},
	}
	h := &SizeBetHandler{settingsService: settingsSvc, gameService: gameSvc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/games/size-bet/rounds/1001/refund", nil)

	h.RefundRound(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(1001), gameSvc.refundRound)
	require.False(t, gameSvc.refundAt.IsZero())

	resp := decodeAdminEnvelope(t, w)
	data := resp.Data.(map[string]any)
	require.Equal(t, float64(2), data["refunded_count"])
}

func TestAdminSizeBetHandlerListRoundsDoesNotLeakOpenRoundSeed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingsSvc := &sizeBetSettingsServiceStub{}
	gameSvc := &sizeBetAdminGameServiceStub{
		rounds: []service.SizeBetRound{
			{
				ID:             1001,
				GameKey:        service.SizeBetGameKey,
				RoundNo:        1001,
				Status:         service.SizeBetRoundStatusOpen,
				ServerSeedHash: "hash-1",
				ServerSeed:     "super-secret-seed",
			},
		},
		roundsPagination: &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 20, Pages: 1},
	}
	h := &SizeBetHandler{settingsService: settingsSvc, gameService: gameSvc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/games/size-bet/rounds", nil)

	h.ListRounds(c)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeAdminEnvelope(t, w)
	data := resp.Data.(map[string]any)
	items := data["items"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	require.Contains(t, item, "round_no")
	require.NotContains(t, item, "GameKey")
	require.NotContains(t, item, "server_seed")
}

func TestAdminSizeBetHandlerListLedgerUsesDTOWithoutGameKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingsSvc := &sizeBetSettingsServiceStub{}
	gameSvc := &sizeBetAdminGameServiceStub{
		ledger: []service.SizeBetLedgerEntry{
			{
				ID:          1,
				UserID:      9,
				GameKey:     service.SizeBetGameKey,
				EntryType:   "bet_debit",
				StakeAmount: 5,
			},
		},
		ledgerPagination: &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 20, Pages: 1},
	}
	h := &SizeBetHandler{settingsService: settingsSvc, gameService: gameSvc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/games/size-bet/ledger", nil)

	h.ListLedger(c)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeAdminEnvelope(t, w)
	data := resp.Data.(map[string]any)
	items := data["items"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	require.Contains(t, item, "entry_type")
	require.NotContains(t, item, "game_key")
	require.NotContains(t, item, "GameKey")
}

func TestAdminSizeBetHandlerListBetsRejectsInvalidUserIDFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingsSvc := &sizeBetSettingsServiceStub{}
	gameSvc := &sizeBetAdminGameServiceStub{}
	h := &SizeBetHandler{settingsService: settingsSvc, gameService: gameSvc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/games/size-bet/bets?user_id=oops", nil)

	h.ListBets(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
