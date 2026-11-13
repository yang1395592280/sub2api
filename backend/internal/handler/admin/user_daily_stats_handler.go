package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type UserDailyStatsHandler struct {
	service *service.UserDailyStatsService
}

func NewUserDailyStatsHandler(s *service.UserDailyStatsService) *UserDailyStatsHandler {
	return &UserDailyStatsHandler{service: s}
}

func dailyStatsFilter(c *gin.Context) service.UserDailyStatsFilter {
	page, size := response.ParsePagination(c)
	return service.UserDailyStatsFilter{
		Date: c.Query("date"), Search: c.Query("search"), Activity: c.Query("activity"),
		SortBy: c.Query("sort_by"), SortOrder: c.Query("sort_order"), Page: page, PageSize: size,
	}
}

func (h *UserDailyStatsHandler) List(c *gin.Context) {
	data, err := h.service.Query(c.Request.Context(), dailyStatsFilter(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}

func (h *UserDailyStatsHandler) Activities(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	data, err := h.service.Activities(c.Request.Context(), userID, dailyStatsFilter(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
