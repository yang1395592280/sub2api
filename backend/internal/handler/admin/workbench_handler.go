package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type WorkbenchHandler struct {
	workbenchService *service.WorkbenchService
}

func NewWorkbenchHandler(workbenchService *service.WorkbenchService) *WorkbenchHandler {
	return &WorkbenchHandler{workbenchService: workbenchService}
}

type adminWorkbenchConversationDetailResponse struct {
	Conversation *service.AdminWorkbenchConversation `json:"conversation"`
	Messages     []service.WorkbenchMessage          `json:"messages"`
}

type adminWorkbenchBatchDeleteRequest struct {
	ConversationIDs []int64 `json:"conversation_ids"`
}

type adminWorkbenchCleanupExpiredRequest struct {
	RetentionDays int `json:"retention_days"`
}

func (h *WorkbenchHandler) GetStats(c *gin.Context) {
	retentionDays := parsePositiveIntQuery(c, "retention_days", 7)
	stats, err := h.workbenchService.AdminGetStats(c.Request.Context(), retentionDays)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

func (h *WorkbenchHandler) ListConversations(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	userID, ok := parseOptionalInt64Query(c, "user_id")
	if !ok {
		return
	}
	hasImages, ok := parseOptionalBoolQuery(c, "has_images")
	if !ok {
		return
	}

	items, pageResult, err := h.workbenchService.AdminListConversations(
		c.Request.Context(),
		pagination.PaginationParams{Page: page, PageSize: pageSize},
		service.AdminWorkbenchConversationFilters{
			Mode:          strings.TrimSpace(c.Query("mode")),
			Status:        strings.TrimSpace(c.Query("status")),
			Search:        strings.TrimSpace(c.Query("search")),
			UserID:        userID,
			HasImages:     hasImages,
			OlderThanDays: parsePositiveIntQuery(c, "older_than_days", 0),
		},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if pageResult != nil {
		response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
		return
	}
	response.Paginated(c, items, 0, page, pageSize)
}

func (h *WorkbenchHandler) GetConversation(c *gin.Context) {
	id, err := parseAdminWorkbenchID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid conversation ID")
		return
	}
	conversation, messages, err := h.workbenchService.AdminGetConversation(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminWorkbenchConversationDetailResponse{
		Conversation: conversation,
		Messages:     messages,
	})
}

func (h *WorkbenchHandler) BatchDeleteConversations(c *gin.Context) {
	var req adminWorkbenchBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	deleted, err := h.workbenchService.AdminHardDeleteConversations(c.Request.Context(), req.ConversationIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": deleted})
}

func (h *WorkbenchHandler) CleanupExpiredConversations(c *gin.Context) {
	var req adminWorkbenchCleanupExpiredRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	deleted, err := h.workbenchService.AdminHardDeleteExpiredConversations(c.Request.Context(), req.RetentionDays)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": deleted})
}

func parseAdminWorkbenchID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

func parsePositiveIntQuery(c *gin.Context, key string, fallback int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func parseOptionalInt64Query(c *gin.Context, key string) (int64, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, true
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed < 0 {
		response.BadRequest(c, "Invalid "+key)
		return 0, false
	}
	return parsed, true
}

func parseOptionalBoolQuery(c *gin.Context, key string) (*bool, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, true
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		response.BadRequest(c, "Invalid "+key)
		return nil, false
	}
	return &parsed, true
}
