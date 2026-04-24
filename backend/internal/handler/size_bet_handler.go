package handler

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type sizeBetService interface {
	GetCurrentRoundView(ctx context.Context, userID int64, now time.Time) (*service.SizeBetCurrentRoundView, error)
	PlaceBet(ctx context.Context, req service.PlaceSizeBetRequest) (*service.SizeBet, error)
	GetHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.SizeBetUserHistoryItem, *pagination.PaginationResult, error)
	ListRounds(ctx context.Context, params pagination.PaginationParams) ([]service.SizeBetRound, *pagination.PaginationResult, error)
	GetLeaderboard(ctx context.Context, scope string, now time.Time) (*service.SizeBetLeaderboardView, error)
	GetRules(ctx context.Context, now time.Time) (*service.SizeBetRulesView, error)
}

type SizeBetHandler struct {
	service sizeBetService
}

type sizeBetCurrentRoundResponse struct {
	ID                  int64   `json:"id"`
	RoundNo             int64   `json:"round_no"`
	Status              string  `json:"status"`
	StartsAt            string  `json:"starts_at"`
	BetClosesAt         string  `json:"bet_closes_at"`
	SettlesAt           string  `json:"settles_at"`
	ProbSmall           float64 `json:"prob_small"`
	ProbMid             float64 `json:"prob_mid"`
	ProbBig             float64 `json:"prob_big"`
	OddsSmall           float64 `json:"odds_small"`
	OddsMid             float64 `json:"odds_mid"`
	OddsBig             float64 `json:"odds_big"`
	AllowedStakes       []int   `json:"allowed_stakes"`
	ServerSeedHash      string  `json:"server_seed_hash"`
	CountdownSeconds    int     `json:"countdown_seconds"`
	BetCountdownSeconds int     `json:"bet_countdown_seconds"`
}

type sizeBetResponse struct {
	ID              int64   `json:"id"`
	RoundID         int64   `json:"round_id"`
	Direction       string  `json:"direction"`
	StakeAmount     float64 `json:"stake_amount"`
	PayoutAmount    float64 `json:"payout_amount"`
	NetResultAmount float64 `json:"net_result_amount"`
	Status          string  `json:"status"`
	PlacedAt        string  `json:"placed_at,omitempty"`
	SettledAt       *string `json:"settled_at,omitempty"`
}

type sizeBetRoundSummaryResponse struct {
	ID              int64   `json:"id"`
	RoundNo         int64   `json:"round_no"`
	Status          string  `json:"status"`
	StartsAt        string  `json:"starts_at"`
	SettlesAt       string  `json:"settles_at"`
	ResultNumber    *int    `json:"result_number,omitempty"`
	ResultDirection string  `json:"result_direction,omitempty"`
	ServerSeedHash  string  `json:"server_seed_hash,omitempty"`
	ServerSeed      *string `json:"server_seed,omitempty"`
}

type sizeBetUserHistoryResponse struct {
	BetID           int64    `json:"bet_id"`
	RoundNo         int64    `json:"round_no"`
	Direction       string   `json:"direction"`
	Selection       string   `json:"selection"`
	ResultNumber    *int     `json:"result_number,omitempty"`
	ResultDirection string   `json:"result_direction,omitempty"`
	StakeAmount     float64  `json:"stake_amount"`
	PayoutAmount    float64  `json:"payout_amount"`
	NetResultAmount float64  `json:"net_result_amount"`
	BalanceAfter    *float64 `json:"balance_after,omitempty"`
	Status          string   `json:"status"`
	PlacedAt        string   `json:"placed_at"`
	SettledAt       *string  `json:"settled_at,omitempty"`
}

type sizeBetCurrentRoundViewResponse struct {
	Enabled       bool                         `json:"enabled"`
	Phase         string                       `json:"phase"`
	ServerTime    string                       `json:"server_time"`
	Round         *sizeBetCurrentRoundResponse `json:"round,omitempty"`
	MyBet         *sizeBetResponse             `json:"my_bet,omitempty"`
	PreviousRound *sizeBetRoundSummaryResponse `json:"previous_round,omitempty"`
}

type PlaceBetRequest struct {
	RoundID        int64                    `json:"round_id" binding:"required"`
	Direction      service.SizeBetDirection `json:"direction" binding:"required"`
	StakeAmount    float64                  `json:"stake_amount" binding:"required"`
	IdempotencyKey string                   `json:"idempotency_key"`
}

func NewSizeBetHandler(sizeBetService *service.SizeBetService) *SizeBetHandler {
	return &SizeBetHandler{service: sizeBetService}
}

func (h *SizeBetHandler) GetCurrent(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.service.GetCurrentRoundView(c.Request.Context(), subject.UserID, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toSizeBetCurrentRoundViewResponse(result))
}

func (h *SizeBetHandler) PlaceBet(c *gin.Context) {
	var req PlaceBetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	bet, err := h.service.PlaceBet(c.Request.Context(), service.PlaceSizeBetRequest{
		UserID:         subject.UserID,
		RoundID:        req.RoundID,
		Direction:      req.Direction,
		StakeAmount:    req.StakeAmount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toSizeBetResponse(bet))
}

func (h *SizeBetHandler) GetHistory(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.service.GetHistory(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var respPagination *response.PaginationResult
	if paginationResult != nil {
		respPagination = &response.PaginationResult{
			Total:    paginationResult.Total,
			Page:     paginationResult.Page,
			PageSize: paginationResult.PageSize,
			Pages:    paginationResult.Pages,
		}
	}
	response.PaginatedWithResult(c, toSizeBetUserHistoryResponses(items), respPagination)
}

func (h *SizeBetHandler) ListRecentRounds(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.service.ListRounds(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var respPagination *response.PaginationResult
	if paginationResult != nil {
		respPagination = &response.PaginationResult{
			Total:    paginationResult.Total,
			Page:     paginationResult.Page,
			PageSize: paginationResult.PageSize,
			Pages:    paginationResult.Pages,
		}
	}
	response.PaginatedWithResult(c, toSizeBetRoundSummaryResponses(items), respPagination)
}

func (h *SizeBetHandler) GetLeaderboard(c *gin.Context) {
	result, err := h.service.GetLeaderboard(c.Request.Context(), c.Query("scope"), time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func toSizeBetCurrentRoundViewResponse(view *service.SizeBetCurrentRoundView) *sizeBetCurrentRoundViewResponse {
	if view == nil {
		return nil
	}
	return &sizeBetCurrentRoundViewResponse{
		Enabled:       view.Enabled,
		Phase:         string(view.Phase),
		ServerTime:    formatTime(view.ServerTime),
		Round:         toSizeBetCurrentRoundResponse(view.Round),
		MyBet:         toSizeBetResponse(view.MyBet),
		PreviousRound: toSizeBetRoundSummaryResponse(view.PreviousRound),
	}
}

func toSizeBetCurrentRoundResponse(round *service.SizeBetCurrentRound) *sizeBetCurrentRoundResponse {
	if round == nil {
		return nil
	}
	return &sizeBetCurrentRoundResponse{
		ID:                  round.ID,
		RoundNo:             round.RoundNo,
		Status:              string(round.Status),
		StartsAt:            formatTime(round.StartsAt),
		BetClosesAt:         formatTime(round.BetClosesAt),
		SettlesAt:           formatTime(round.SettlesAt),
		ProbSmall:           round.ProbSmall,
		ProbMid:             round.ProbMid,
		ProbBig:             round.ProbBig,
		OddsSmall:           round.OddsSmall,
		OddsMid:             round.OddsMid,
		OddsBig:             round.OddsBig,
		AllowedStakes:       append([]int(nil), round.AllowedStakes...),
		ServerSeedHash:      round.ServerSeedHash,
		CountdownSeconds:    round.CountdownSeconds,
		BetCountdownSeconds: round.BetCountdownSeconds,
	}
}

func toSizeBetResponse(bet *service.SizeBet) *sizeBetResponse {
	if bet == nil {
		return nil
	}
	resp := &sizeBetResponse{
		ID:              bet.ID,
		RoundID:         bet.RoundID,
		Direction:       string(bet.Direction),
		StakeAmount:     bet.StakeAmount,
		PayoutAmount:    bet.PayoutAmount,
		NetResultAmount: bet.NetResultAmount,
		Status:          string(bet.Status),
		PlacedAt:        formatTime(bet.PlacedAt),
	}
	if bet.SettledAt != nil {
		settledAt := formatTime(*bet.SettledAt)
		resp.SettledAt = &settledAt
	}
	return resp
}

func toSizeBetRoundSummaryResponses(rounds []service.SizeBetRound) []sizeBetRoundSummaryResponse {
	items := make([]sizeBetRoundSummaryResponse, 0, len(rounds))
	for i := range rounds {
		if item := toSizeBetRoundSummaryResponse(&rounds[i]); item != nil {
			items = append(items, *item)
		}
	}
	return items
}

func toSizeBetRoundSummaryResponse(round *service.SizeBetRound) *sizeBetRoundSummaryResponse {
	if round == nil {
		return nil
	}
	resp := &sizeBetRoundSummaryResponse{
		ID:             round.ID,
		RoundNo:        round.RoundNo,
		Status:         string(round.Status),
		StartsAt:       formatTime(round.StartsAt),
		SettlesAt:      formatTime(round.SettlesAt),
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

func toSizeBetUserHistoryResponses(items []service.SizeBetUserHistoryItem) []sizeBetUserHistoryResponse {
	respItems := make([]sizeBetUserHistoryResponse, 0, len(items))
	for i := range items {
		respItems = append(respItems, toSizeBetUserHistoryResponse(items[i]))
	}
	return respItems
}

func toSizeBetUserHistoryResponse(item service.SizeBetUserHistoryItem) sizeBetUserHistoryResponse {
	resp := sizeBetUserHistoryResponse{
		BetID:           item.BetID,
		RoundNo:         item.RoundNo,
		Direction:       string(item.Direction),
		Selection:       string(item.Direction),
		ResultNumber:    item.ResultNumber,
		StakeAmount:     item.StakeAmount,
		PayoutAmount:    item.PayoutAmount,
		NetResultAmount: item.NetResultAmount,
		BalanceAfter:    item.BalanceAfter,
		Status:          string(item.Status),
		PlacedAt:        formatTime(item.PlacedAt),
	}
	if item.ResultDirection != "" {
		resp.ResultDirection = string(item.ResultDirection)
	}
	if item.SettledAt != nil {
		settledAt := formatTime(*item.SettledAt)
		resp.SettledAt = &settledAt
	}
	return resp
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func (h *SizeBetHandler) GetRules(c *gin.Context) {
	result, err := h.service.GetRules(c.Request.Context(), time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
