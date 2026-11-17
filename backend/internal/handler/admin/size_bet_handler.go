package admin

import (
	"context"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type sizeBetSettingsService interface {
	GetSettings(ctx context.Context) (*service.SizeBetSettings, error)
	UpdateSettings(ctx context.Context, req service.UpdateSizeBetSettingsRequest) error
}

type sizeBetAdminGameService interface {
	ListRounds(ctx context.Context, params pagination.PaginationParams) ([]service.SizeBetRound, *pagination.PaginationResult, error)
	ListBets(ctx context.Context, params pagination.PaginationParams, filter service.SizeBetAdminBetFilter) ([]service.SizeBetAdminBet, *pagination.PaginationResult, error)
	ListLedger(ctx context.Context, params pagination.PaginationParams, filter service.SizeBetAdminLedgerFilter) ([]service.SizeBetLedgerEntry, *pagination.PaginationResult, error)
	RefundRound(ctx context.Context, roundID int64, refundedAt time.Time) (*service.SizeBetRefundResult, error)
	GetStatsOverview(ctx context.Context, date string) (*service.SizeBetStatsOverview, error)
	ListStatsUsers(ctx context.Context, date string, params pagination.PaginationParams) ([]service.SizeBetStatsUserItem, *pagination.PaginationResult, error)
}

type SizeBetHandler struct {
	settingsService sizeBetSettingsService
	gameService     sizeBetAdminGameService
}

type sizeBetAdminRoundResponse struct {
	ID              int64   `json:"id"`
	RoundNo         int64   `json:"round_no"`
	Status          string  `json:"status"`
	StartsAt        string  `json:"starts_at"`
	BetClosesAt     string  `json:"bet_closes_at"`
	SettlesAt       string  `json:"settles_at"`
	ProbSmall       float64 `json:"prob_small"`
	ProbMid         float64 `json:"prob_mid"`
	ProbBig         float64 `json:"prob_big"`
	OddsSmall       float64 `json:"odds_small"`
	OddsMid         float64 `json:"odds_mid"`
	OddsBig         float64 `json:"odds_big"`
	AllowedStakes   []int   `json:"allowed_stakes"`
	ResultNumber    *int    `json:"result_number,omitempty"`
	ResultDirection string  `json:"result_direction,omitempty"`
	ServerSeedHash  string  `json:"server_seed_hash,omitempty"`
	ServerSeed      *string `json:"server_seed,omitempty"`
}

type sizeBetAdminBetResponse struct {
	ID              int64   `json:"id"`
	RoundID         int64   `json:"round_id"`
	RoundNo         int64   `json:"round_no"`
	UserID          int64   `json:"user_id"`
	Username        string  `json:"username"`
	Direction       string  `json:"direction"`
	StakeAmount     float64 `json:"stake_amount"`
	PayoutAmount    float64 `json:"payout_amount"`
	NetResultAmount float64 `json:"net_result_amount"`
	Status          string  `json:"status"`
	PlacedAt        string  `json:"placed_at,omitempty"`
	SettledAt       *string `json:"settled_at,omitempty"`
}

type sizeBetAdminLedgerResponse struct {
	ID            int64   `json:"id"`
	UserID        int64   `json:"user_id"`
	RoundID       *int64  `json:"round_id,omitempty"`
	BetID         *int64  `json:"bet_id,omitempty"`
	EntryType     string  `json:"entry_type"`
	Direction     string  `json:"direction,omitempty"`
	StakeAmount   float64 `json:"stake_amount"`
	DeltaAmount   float64 `json:"delta_amount"`
	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`
	Reason        string  `json:"reason,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
}

func NewSizeBetHandler(settingsService *service.SizeBetAdminService, gameService *service.SizeBetService) *SizeBetHandler {
	return &SizeBetHandler{
		settingsService: settingsService,
		gameService:     gameService,
	}
}

func (h *SizeBetHandler) GetSettings(c *gin.Context) {
	result, err := h.settingsService.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *SizeBetHandler) UpdateSettings(c *gin.Context) {
	var req service.UpdateSizeBetSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.settingsService.UpdateSettings(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "updated"})
}

func (h *SizeBetHandler) ListRounds(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.gameService.ListRounds(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	writePaginated(c, toSizeBetAdminRoundResponses(items), paginationResult)
}

func (h *SizeBetHandler) ListBets(c *gin.Context) {
	roundID, ok := parseOptionalInt64Query(c, "round_id")
	if !ok {
		return
	}
	userID, ok := parseOptionalInt64Query(c, "user_id")
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.gameService.ListBets(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, service.SizeBetAdminBetFilter{
		RoundID: roundID,
		UserID:  userID,
		Status:  c.Query("status"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	writePaginated(c, toSizeBetAdminBetResponses(items), paginationResult)
}

func (h *SizeBetHandler) ListLedger(c *gin.Context) {
	roundID, ok := parseOptionalInt64Query(c, "round_id")
	if !ok {
		return
	}
	userID, ok := parseOptionalInt64Query(c, "user_id")
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.gameService.ListLedger(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, service.SizeBetAdminLedgerFilter{
		RoundID:   roundID,
		UserID:    userID,
		EntryType: c.Query("entry_type"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	writePaginated(c, toSizeBetAdminLedgerResponses(items), paginationResult)
}

func (h *SizeBetHandler) RefundRound(c *gin.Context) {
	roundID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	result, err := h.gameService.RefundRound(c.Request.Context(), roundID, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *SizeBetHandler) GetStatsOverview(c *gin.Context) {
	result, err := h.gameService.GetStatsOverview(c.Request.Context(), c.Query("date"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *SizeBetHandler) ListStatsUsers(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.gameService.ListStatsUsers(c.Request.Context(), c.Query("date"), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	writePaginated(c, items, paginationResult)
}

func writePaginated(c *gin.Context, items any, paginationResult *pagination.PaginationResult) {
	if paginationResult == nil {
		paginationResult = &pagination.PaginationResult{
			Total:    0,
			Page:     1,
			PageSize: 20,
			Pages:    1,
		}
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{
		Total:    paginationResult.Total,
		Page:     paginationResult.Page,
		PageSize: paginationResult.PageSize,
		Pages:    paginationResult.Pages,
	})
}

func parseOptionalInt64Query(c *gin.Context, key string) (*int64, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid "+key)
		return nil, false
	}
	return &parsed, true
}

func toSizeBetAdminRoundResponses(rounds []service.SizeBetRound) []sizeBetAdminRoundResponse {
	items := make([]sizeBetAdminRoundResponse, 0, len(rounds))
	for i := range rounds {
		if item := toSizeBetAdminRoundResponse(&rounds[i]); item != nil {
			items = append(items, *item)
		}
	}
	return items
}

func toSizeBetAdminRoundResponse(round *service.SizeBetRound) *sizeBetAdminRoundResponse {
	if round == nil {
		return nil
	}
	resp := &sizeBetAdminRoundResponse{
		ID:             round.ID,
		RoundNo:        round.RoundNo,
		Status:         string(round.Status),
		StartsAt:       formatAdminTime(round.StartsAt),
		BetClosesAt:    formatAdminTime(round.BetClosesAt),
		SettlesAt:      formatAdminTime(round.SettlesAt),
		ProbSmall:      round.ProbSmall,
		ProbMid:        round.ProbMid,
		ProbBig:        round.ProbBig,
		OddsSmall:      round.OddsSmall,
		OddsMid:        round.OddsMid,
		OddsBig:        round.OddsBig,
		AllowedStakes:  append([]int(nil), round.AllowedStakes...),
		ResultNumber:   round.ResultNumber,
		ServerSeedHash: round.ServerSeedHash,
	}
	if round.ResultDirection != "" {
		resp.ResultDirection = string(round.ResultDirection)
	}
	if round.Status == service.SizeBetRoundStatusSettled && round.ResultNumber != nil && round.ServerSeed != "" {
		serverSeed := round.ServerSeed
		resp.ServerSeed = &serverSeed
	}
	return resp
}

func toSizeBetAdminBetResponses(bets []service.SizeBetAdminBet) []sizeBetAdminBetResponse {
	items := make([]sizeBetAdminBetResponse, 0, len(bets))
	for i := range bets {
		items = append(items, sizeBetAdminBetResponse{
			ID:              bets[i].ID,
			RoundID:         bets[i].RoundID,
			RoundNo:         bets[i].RoundNo,
			UserID:          bets[i].UserID,
			Username:        bets[i].Username,
			Direction:       string(bets[i].Direction),
			StakeAmount:     bets[i].StakeAmount,
			PayoutAmount:    bets[i].PayoutAmount,
			NetResultAmount: bets[i].NetResultAmount,
			Status:          string(bets[i].Status),
			PlacedAt:        formatAdminTime(bets[i].PlacedAt),
			SettledAt:       formatAdminOptionalTime(bets[i].SettledAt),
		})
	}
	return items
}

func toSizeBetAdminLedgerResponses(entries []service.SizeBetLedgerEntry) []sizeBetAdminLedgerResponse {
	items := make([]sizeBetAdminLedgerResponse, 0, len(entries))
	for i := range entries {
		items = append(items, sizeBetAdminLedgerResponse{
			ID:            entries[i].ID,
			UserID:        entries[i].UserID,
			RoundID:       entries[i].RoundID,
			BetID:         entries[i].BetID,
			EntryType:     entries[i].EntryType,
			Direction:     entries[i].Direction,
			StakeAmount:   entries[i].StakeAmount,
			DeltaAmount:   entries[i].DeltaAmount,
			BalanceBefore: entries[i].BalanceBefore,
			BalanceAfter:  entries[i].BalanceAfter,
			Reason:        entries[i].Reason,
			CreatedAt:     formatAdminTime(entries[i].CreatedAt),
		})
	}
	return items
}

func formatAdminOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := formatAdminTime(*t)
	return &formatted
}

func formatAdminTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
