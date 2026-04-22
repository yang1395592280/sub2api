package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

type CheckinRequest struct {
	TurnstileToken string `json:"turnstile_token"`
	Timezone       string `json:"timezone"`
}

type CheckinResponse struct {
	CheckinDate      string  `json:"checkin_date"`
	RewardAmount     float64 `json:"reward_amount"`
	BaseRewardAmount float64 `json:"base_reward_amount,omitempty"`
	BonusStatus      string  `json:"bonus_status,omitempty"`
	BonusDeltaAmount float64 `json:"bonus_delta_amount,omitempty"`
}

func (h *UserHandler) GetCheckinStatus(c *gin.Context) {
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

func (h *UserHandler) DoCheckin(c *gin.Context) {
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
		RewardAmount:     record.RewardAmount,
		BaseRewardAmount: record.BaseRewardAmount,
		BonusStatus:      record.BonusStatus,
		BonusDeltaAmount: record.BonusDeltaAmount,
	})
}

func (h *UserHandler) PlayCheckinLuckyBonus(c *gin.Context) {
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
		RewardAmount:     record.RewardAmount,
		BaseRewardAmount: record.BaseRewardAmount,
		BonusStatus:      record.BonusStatus,
		BonusDeltaAmount: record.BonusDeltaAmount,
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
