package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type CheckinHandler struct {
	checkinService *service.CheckinService
}

func (h *CheckinHandler) Settings(c *gin.Context) {
	if c.Request.Method == "GET" {
		response.Success(c, h.checkinService.GetSettings(c.Request.Context()))
		return
	}
	var req service.CheckinSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid settings")
		return
	}
	if err := h.checkinService.UpdateSettings(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.checkinService.GetSettings(c.Request.Context()))
}

func (h *CheckinHandler) Risks(c *gin.Context) {
	page, size := response.ParsePagination(c)
	items, total, err := h.checkinService.ListRisks(c.Request.Context(), page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}

func (h *CheckinHandler) ConfirmRisk(c *gin.Context) {
	var req struct {
		UserID   int64  `json:"user_id" binding:"required,gt=0"`
		Reason   string `json:"reason"`
		Evidence string `json:"evidence"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.BadRequest(c, "Invalid risk case")
		return
	}
	var actorID int64
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		actorID = subject.UserID
	}
	data, err := h.checkinService.ConfirmRisk(c.Request.Context(), req.UserID, actorID, req.Reason, req.Evidence)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}

func (h *CheckinHandler) ResolveRisk(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid case ID")
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.BadRequest(c, "Invalid action")
		return
	}
	var actorID int64
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		actorID = subject.UserID
	}
	if err = h.checkinService.ResolveRisk(c.Request.Context(), id, actorID, req.Action); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

func NewCheckinHandler(checkinService *service.CheckinService) *CheckinHandler {
	return &CheckinHandler{checkinService: checkinService}
}

func (h *CheckinHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	date := strings.TrimSpace(c.Query("date"))
	timezone := strings.TrimSpace(c.Query("timezone"))
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	items, total, err := h.checkinService.ListAdminRecords(c.Request.Context(), page, pageSize, search, date, timezone, sortBy, sortOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Paginated(c, items, total, page, pageSize)
}

func (h *CheckinHandler) Analytics(c *gin.Context) {
	topLimit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("top_limit", "10")))

	data, err := h.checkinService.GetAdminAnalytics(c.Request.Context(), service.AdminCheckinAnalyticsFilter{
		StartDate: strings.TrimSpace(c.Query("start_date")),
		EndDate:   strings.TrimSpace(c.Query("end_date")),
		Search:    strings.TrimSpace(c.Query("search")),
		Timezone:  strings.TrimSpace(c.Query("timezone")),
		TopLimit:  topLimit,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, data)
}

func (h *CheckinHandler) GetUserSummary(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	status, err := h.checkinService.GetStatus(c.Request.Context(), userID, c.Query("month"), c.Query("timezone"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, status)
}
