package handler

import (
	"context"
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type zenxiangLiyuUserService interface {
	GetStatus(ctx context.Context, userID int64) (*service.ZenxiangLiyuStatus, error)
	Play(ctx context.Context, userID int64, requestID string) (*service.ZenxiangLiyuPlayResult, error)
	PlayLuckyCoin(ctx context.Context, userID, recordID int64) (*service.ZenxiangLiyuLuckyCoinResult, error)
	PlayGuessSize(ctx context.Context, userID, recordID int64, choice string) (*service.ZenxiangLiyuGuessSizeResult, error)
	ListUserRecords(ctx context.Context, userID int64, page, pageSize int) ([]service.ZenxiangLiyuRecord, int, error)
	GetUserDailySummary(ctx context.Context, userID int64) (*service.ZenxiangLiyuDailySummary, error)
}

func (h *ZenxiangLiyuHandler) ListRecords(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	records, total, err := h.service.ListUserRecords(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.InternalError(c, "Failed to get Zenxiang Liyu records")
		return
	}
	response.Paginated(c, records, int64(total), page, pageSize)
}

func (h *ZenxiangLiyuHandler) GetDailySummary(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	summary, err := h.service.GetUserDailySummary(c.Request.Context(), subject.UserID)
	if err != nil {
		response.InternalError(c, "Failed to get Zenxiang Liyu daily summary")
		return
	}
	response.Success(c, summary)
}

// ZenxiangLiyuHandler handles the authenticated user-facing Zenxiang Liyu APIs.
type ZenxiangLiyuHandler struct {
	service zenxiangLiyuUserService
}

func NewZenxiangLiyuHandler(service zenxiangLiyuUserService) *ZenxiangLiyuHandler {
	return &ZenxiangLiyuHandler{service: service}
}

type zenxiangLiyuPlayRequest struct {
	RequestID string `json:"request_id" binding:"required"`
}

type zenxiangLiyuGuessSizeRequest struct {
	Choice string `json:"choice" binding:"required,oneof=big small skip"`
}

// GetStatus returns the current user's eligibility and prize display data.
func (h *ZenxiangLiyuHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	status, err := h.service.GetStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.InternalError(c, "Failed to get Zenxiang Liyu status")
		return
	}
	response.Success(c, status)
}

// Play executes a server-configured Zenxiang Liyu play for the current user.
func (h *ZenxiangLiyuHandler) Play(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req zenxiangLiyuPlayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.service.Play(c.Request.Context(), subject.UserID, req.RequestID)
	if err != nil {
		handleZenxiangLiyuPlayError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ZenxiangLiyuHandler) PlayLuckyCoin(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	recordID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || recordID <= 0 {
		response.BadRequest(c, "Invalid record ID")
		return
	}
	result, err := h.service.PlayLuckyCoin(c.Request.Context(), subject.UserID, recordID)
	if err != nil {
		handleZenxiangLiyuPlayError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ZenxiangLiyuHandler) PlayGuessSize(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	recordID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || recordID <= 0 {
		response.BadRequest(c, "Invalid record ID")
		return
	}
	var req zenxiangLiyuGuessSizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.PlayGuessSize(c.Request.Context(), subject.UserID, recordID, req.Choice)
	if err != nil {
		handleZenxiangLiyuPlayError(c, err)
		return
	}
	response.Success(c, result)
}

func handleZenxiangLiyuPlayError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrZenxiangLiyuDisabled), errors.Is(err, service.ErrZenxiangLiyuUnauthorized):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrZenxiangLiyuRequestIDRequired),
		errors.Is(err, service.ErrZenxiangLiyuInsufficientBalance),
		errors.Is(err, service.ErrZenxiangLiyuDailyLimitReached),
		errors.Is(err, service.ErrZenxiangLiyuNoTicket),
		errors.Is(err, service.ErrZenxiangLiyuLuckyCoinDisabled),
		errors.Is(err, service.ErrZenxiangLiyuLuckyCoinAlreadyPlayed),
		errors.Is(err, service.ErrZenxiangLiyuLuckyCoinUnavailable),
		errors.Is(err, service.ErrZenxiangLiyuGuessSizeDisabled),
		errors.Is(err, service.ErrZenxiangLiyuGuessSizeUnavailable),
		errors.Is(err, service.ErrZenxiangLiyuGuessSizeAlreadyPlayed),
		errors.Is(err, service.ErrZenxiangLiyuGuessSizeInvalidChoice):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "Failed to play Zenxiang Liyu")
	}
}
