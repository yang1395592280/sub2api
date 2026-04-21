package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type CheckinHandler struct {
	checkinService *service.CheckinService
}

func NewCheckinHandler(checkinService *service.CheckinService) *CheckinHandler {
	return &CheckinHandler{checkinService: checkinService}
}

func (h *CheckinHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	date := strings.TrimSpace(c.Query("date"))
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	items, total, err := h.checkinService.ListAdminRecords(c.Request.Context(), page, pageSize, search, date, sortBy, sortOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Paginated(c, items, total, page, pageSize)
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
