package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type UserCheckinHandler struct {
	checkinService   *service.CheckinService
	turnstileService *service.TurnstileService
}

type CheckinRequest struct {
	TurnstileToken string `json:"turnstile_token"`
	Timezone       string `json:"timezone"`
}

type CheckinResponse struct {
	CheckinDate      string  `json:"checkin_date"`
	RewardPoints     int64 `json:"reward_points"`
	BaseRewardPoints int64 `json:"base_reward_points,omitempty"`
	BonusStatus      string  `json:"bonus_status,omitempty"`
	BonusDeltaPoints int64 `json:"bonus_delta_points,omitempty"`
}

func NewUserCheckinHandler(checkinService *service.CheckinService, turnstileService *service.TurnstileService) *UserCheckinHandler {
	return &UserCheckinHandler{
		checkinService:   checkinService,
		turnstileService: turnstileService,
	}
}

func (h *UserCheckinHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	status, err := h.checkinService.GetStatus(
		c.Request.Context(),
		subject.UserID,
		c.Query("month"),
		c.Query("timezone"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, status)
}

func (h *UserCheckinHandler) Checkin(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req CheckinRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}

	if h.turnstileService != nil {
		if err := h.turnstileService.VerifyToken(c.Request.Context(), req.TurnstileToken, ip.GetClientIP(c)); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	record, err := h.checkinService.Checkin(
		c.Request.Context(),
		subject.UserID,
		resolveCheckinTimezone(req.Timezone, c.Query("timezone")),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, CheckinResponse{
		CheckinDate:      record.CheckinDate,
		RewardPoints:     record.RewardPoints,
		BaseRewardPoints: record.BaseRewardPoints,
		BonusStatus:      record.BonusStatus,
		BonusDeltaPoints: record.BonusDeltaPoints,
	})
}

func (h *UserCheckinHandler) PlayLuckyBonus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req CheckinRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}

	record, err := h.checkinService.PlayLuckyBonus(
		c.Request.Context(),
		subject.UserID,
		resolveCheckinTimezone(req.Timezone, c.Query("timezone")),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, CheckinResponse{
		CheckinDate:      record.CheckinDate,
		RewardPoints:     record.RewardPoints,
		BaseRewardPoints: record.BaseRewardPoints,
		BonusStatus:      record.BonusStatus,
		BonusDeltaPoints: record.BonusDeltaPoints,
	})
}

func resolveCheckinTimezone(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
