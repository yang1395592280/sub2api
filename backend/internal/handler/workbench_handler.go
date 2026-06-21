package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type WorkbenchHandler struct {
	workbenchService *service.WorkbenchService
}

func NewWorkbenchHandler(workbenchService *service.WorkbenchService) *WorkbenchHandler {
	return &WorkbenchHandler{workbenchService: workbenchService}
}

type createWorkbenchConversationRequest struct {
	Mode     string `json:"mode"`
	Title    string `json:"title"`
	APIKeyID *int64 `json:"api_key_id"`
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
}

type workbenchSendRequest struct {
	Mode     string         `json:"mode" binding:"required"`
	APIKeyID int64          `json:"api_key_id" binding:"required"`
	Endpoint string         `json:"endpoint" binding:"required"`
	Model    string         `json:"model" binding:"required"`
	Input    string         `json:"input" binding:"required"`
	Options  map[string]any `json:"options"`
}

func (h *WorkbenchHandler) ListConversations(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	items, pageResult, err := h.workbenchService.ListConversations(
		c.Request.Context(),
		subject.UserID,
		pagination.PaginationParams{Page: page, PageSize: pageSize},
		service.WorkbenchConversationFilters{Mode: strings.TrimSpace(c.Query("mode"))},
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

func (h *WorkbenchHandler) CreateConversation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req createWorkbenchConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	conv, err := h.workbenchService.CreateConversation(c.Request.Context(), subject.UserID, service.CreateWorkbenchConversationRequest{
		Mode:     req.Mode,
		Title:    req.Title,
		APIKeyID: req.APIKeyID,
		Endpoint: req.Endpoint,
		Model:    req.Model,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Created(c, conv)
}

func (h *WorkbenchHandler) ListMessages(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := parseWorkbenchID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid conversation ID")
		return
	}

	items, err := h.workbenchService.ListMessages(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, items)
}

func (h *WorkbenchHandler) DeleteConversation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := parseWorkbenchID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid conversation ID")
		return
	}

	if err := h.workbenchService.DeleteConversation(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "ok"})
}

func (h *WorkbenchHandler) Send(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := parseWorkbenchID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid conversation ID")
		return
	}

	var req workbenchSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.workbenchService.Send(c.Request.Context(), subject.UserID, id, service.WorkbenchSendRequest{
		Mode:     req.Mode,
		APIKeyID: req.APIKeyID,
		Endpoint: req.Endpoint,
		Model:    req.Model,
		Input:    req.Input,
		Options:  req.Options,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

func parseWorkbenchID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}
