package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gameCenterAdminServiceStub struct {
	settings    *service.GameCenterAdminSettings
	settingsErr error
	updateReq   service.GameCenterAdminSettings
	updateErr   error

	catalog          []service.GameCatalog
	catalogErr       error
	updateCatalogKey string
	updateCatalogReq service.UpdateGameCatalogRequest
	updateCatalogErr error

	ledgerItems  []service.GameCenterAdminLedgerItem
	ledgerPage   *pagination.PaginationResult
	ledgerErr    error
	ledgerUserID *int64

	claimItems  []service.GameCenterClaimRecord
	claimPage   *pagination.PaginationResult
	claimErr    error
	claimUserID *int64

	exchangeItems  []service.GameCenterExchangeRecord
	exchangePage   *pagination.PaginationResult
	exchangeErr    error
	exchangeUserID *int64

	adjustInput service.AdminAdjustPointsInput
	adjustErr   error
}

func (s *gameCenterAdminServiceStub) GetAdminSettings(context.Context) (*service.GameCenterAdminSettings, error) {
	return s.settings, s.settingsErr
}

func (s *gameCenterAdminServiceStub) UpdateAdminSettings(_ context.Context, req service.GameCenterAdminSettings) error {
	s.updateReq = req
	return s.updateErr
}

func (s *gameCenterAdminServiceStub) GetCatalog(context.Context) ([]service.GameCatalog, error) {
	return s.catalog, s.catalogErr
}

func (s *gameCenterAdminServiceStub) UpdateCatalog(_ context.Context, gameKey string, req service.UpdateGameCatalogRequest) error {
	s.updateCatalogKey = gameKey
	s.updateCatalogReq = req
	return s.updateCatalogErr
}

func (s *gameCenterAdminServiceStub) GetAdminLedger(_ context.Context, _ pagination.PaginationParams, userID *int64) ([]service.GameCenterAdminLedgerItem, *pagination.PaginationResult, error) {
	s.ledgerUserID = userID
	return s.ledgerItems, s.ledgerPage, s.ledgerErr
}

func (s *gameCenterAdminServiceStub) GetClaimRecords(_ context.Context, _ pagination.PaginationParams, userID *int64) ([]service.GameCenterClaimRecord, *pagination.PaginationResult, error) {
	s.claimUserID = userID
	return s.claimItems, s.claimPage, s.claimErr
}

func (s *gameCenterAdminServiceStub) GetExchangeRecords(_ context.Context, _ pagination.PaginationParams, userID *int64) ([]service.GameCenterExchangeRecord, *pagination.PaginationResult, error) {
	s.exchangeUserID = userID
	return s.exchangeItems, s.exchangePage, s.exchangeErr
}

func (s *gameCenterAdminServiceStub) AdjustPoints(_ context.Context, input service.AdminAdjustPointsInput) error {
	s.adjustInput = input
	return s.adjustErr
}

func decodeGameCenterAdminEnvelope(t *testing.T, w *httptest.ResponseRecorder) response.Response {
	t.Helper()
	var resp response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func TestGameCenterHandlerGetSettingsReturnsStructuredPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &gameCenterAdminServiceStub{
		settings: &service.GameCenterAdminSettings{
			GameCenterEnabled: true,
			ClaimEnabled:      true,
			ClaimSchedule: []service.GameCenterClaimBatchConfig{{
				BatchKey:     "night",
				ClaimTime:    "20:00",
				PointsAmount: 100,
				Enabled:      true,
			}},
			Exchange: service.GameCenterExchangeSettings{
				BalanceToPointsEnabled: true,
				PointsToBalanceEnabled: true,
				BalanceToPointsRate:    100,
				PointsToBalanceRate:    0.01,
				MinBalanceAmount:       1,
				MinPointsAmount:        100,
			},
		},
	}
	h := &GameCenterHandler{service: svc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/game-center/settings", nil)

	h.GetSettings(c)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeGameCenterAdminEnvelope(t, w)
	data := resp.Data.(map[string]any)
	require.Equal(t, true, data["game_center_enabled"])
	require.Equal(t, true, data["claim_enabled"])
}

func TestGameCenterHandlerUpdateCatalogPersistsEditableFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &gameCenterAdminServiceStub{}
	h := &GameCenterHandler{service: svc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "gameKey", Value: "size_bet"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/game-center/catalog/size_bet", strings.NewReader(`{
		"enabled": true,
		"sort_order": 2,
		"default_open_mode": "dual",
		"supports_embed": true,
		"supports_standalone": true
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateCatalog(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "size_bet", svc.updateCatalogKey)
	require.Equal(t, 2, svc.updateCatalogReq.SortOrder)
	require.Equal(t, "dual", svc.updateCatalogReq.DefaultOpenMode)
}

func TestGameCenterHandlerListLedgerReturnsPaginatedItems(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &gameCenterAdminServiceStub{
		ledgerItems: []service.GameCenterAdminLedgerItem{{ID: 1, UserID: 7, EntryType: "admin_adjust", DeltaPoints: 20}},
		ledgerPage:  &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 20, Pages: 1},
	}
	h := &GameCenterHandler{service: svc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/game-center/ledger?user_id=7", nil)

	h.ListLedger(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.ledgerUserID)
	require.Equal(t, int64(7), *svc.ledgerUserID)
}

func TestGameCenterHandlerAdjustPointsPersistsPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &gameCenterAdminServiceStub{}
	h := &GameCenterHandler{service: svc}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/game-center/users/7/points/adjust", strings.NewReader(`{
		"delta_points": 25,
		"reason": "运营补发"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.AdjustPoints(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(7), svc.adjustInput.UserID)
	require.Equal(t, int64(25), svc.adjustInput.DeltaPoints)
	require.Equal(t, "运营补发", svc.adjustInput.Reason)
}
