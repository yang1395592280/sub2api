package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CheckinHandler struct {
	checkin *service.CheckinService
	auth    *service.AuthService
}

func NewCheckinHandler(checkin *service.CheckinService, auth *service.AuthService) *CheckinHandler {
	return &CheckinHandler{checkin: checkin, auth: auth}
}

func (h *CheckinHandler) Status(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status, err := h.checkin.GetStatus(c.Request.Context(), subject.UserID, c.Query("month"), "Asia/Shanghai")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *CheckinHandler) Challenge(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	token, err := h.checkin.CreatePageToken(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"token": token})
}

func (h *CheckinHandler) Claim(c *gin.Context) { h.play(c, false) }
func (h *CheckinHandler) Bonus(c *gin.Context) { h.play(c, true) }

func (h *CheckinHandler) play(c *gin.Context, bonus bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		Token          string `json:"page_token"`
		Website        string `json:"website"`
		TurnstileToken string `json:"turnstile_token"`
		TencentTicket  string `json:"tencent_ticket"`
		TencentRandstr string `json:"tencent_randstr"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.BadRequest(c, "Invalid check-in request")
		return
	}
	ctx := c.Request.Context()
	ip := middleware.SecurityClientIP(c)
	record := func(event string) bool {
		if err := h.checkin.SecurityEvent(ctx, subject.UserID, event, ip, c.Request.UserAgent()); err != nil {
			response.ErrorFrom(c, err)
			return false
		}
		return true
	}
	if req.Website != "" && !record("honeypot") {
		return
	}
	valid, err := h.checkin.CheckPageToken(ctx, subject.UserID, req.Token)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !valid && !record("page_bypass") {
		return
	}
	if err = h.auth.VerifyCaptcha(ctx, service.CaptchaProof{TurnstileToken: req.TurnstileToken, TencentTicket: req.TencentTicket, TencentRandstr: req.TencentRandstr}, ip); err != nil {
		// Only a direct proof rejection is evidence. Misconfiguration and wrapped
		// verifier/network errors must never count toward a user's confiscation.
		switch err {
		case service.ErrTurnstileVerificationFailed, service.ErrTencentCaptchaVerificationFailed, service.ErrAliyunCaptchaVerificationFailed:
			if !record("captcha_failure") {
				return
			}
		}
		response.ErrorFrom(c, err)
		return
	}
	var result *service.CheckinRecord
	if bonus {
		result, err = h.checkin.PlayLuckyBonus(ctx, subject.UserID, "Asia/Shanghai")
	} else {
		result, err = h.checkin.Checkin(ctx, subject.UserID, "Asia/Shanghai")
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
