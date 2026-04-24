package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type windsurfAccountService interface {
	List(ctx context.Context, params pagination.PaginationParams, filters service.WindsurfAccountListFilters) ([]service.WindsurfAccountListItem, *pagination.PaginationResult, error)
	Create(ctx context.Context, input *service.CreateWindsurfAccountInput) (*service.WindsurfAccountListItem, error)
	UpdateCredentials(ctx context.Context, id int64, input *service.UpdateWindsurfAccountCredentialsInput) (*service.WindsurfAccountListItem, error)
	UpdateStatus(ctx context.Context, id int64, input *service.UpdateWindsurfAccountStatusInput) (*service.WindsurfAccountListItem, error)
	RevealPassword(ctx context.Context, id int64, input *service.RevealWindsurfAccountPasswordInput) (string, error)
	Delete(ctx context.Context, id int64, input *service.DeleteWindsurfAccountInput) error
}

type WindsurfAccountHandler struct {
	service windsurfAccountService
}

func NewWindsurfAccountHandler(service windsurfAccountService) *WindsurfAccountHandler {
	return &WindsurfAccountHandler{service: service}
}

type createWindsurfAccountRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type updateWindsurfAccountCredentialsRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password"`
}

type updateWindsurfAccountStatusRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *WindsurfAccountHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.service.List(c.Request.Context(), pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "maintained_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}, service.WindsurfAccountListFilters{
		Search: strings.TrimSpace(c.Query("search")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Paginated(c, items, paginationResult.Total, page, pageSize)
}

func (h *WindsurfAccountHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req createWindsurfAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	item, err := h.service.Create(c.Request.Context(), &service.CreateWindsurfAccountInput{
		Account:  req.Account,
		Password: req.Password,
		ActorID:  subject.UserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *WindsurfAccountHandler) UpdateCredentials(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid windsurf account ID")
		return
	}

	var req updateWindsurfAccountCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	item, err := h.service.UpdateCredentials(c.Request.Context(), id, &service.UpdateWindsurfAccountCredentialsInput{
		Account:  req.Account,
		Password: req.Password,
		ActorID:  subject.UserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *WindsurfAccountHandler) UpdateStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	role, _ := middleware2.GetUserRoleFromContext(c)
	if role != service.RoleAdmin {
		response.Forbidden(c, "Only admin can update windsurf account status")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid windsurf account ID")
		return
	}

	var req updateWindsurfAccountStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	item, err := h.service.UpdateStatus(c.Request.Context(), id, &service.UpdateWindsurfAccountStatusInput{
		Enabled: req.Enabled,
		ActorID: subject.UserID,
		IsAdmin: true,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *WindsurfAccountHandler) RevealPassword(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	role, _ := middleware2.GetUserRoleFromContext(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid windsurf account ID")
		return
	}

	password, err := h.service.RevealPassword(c.Request.Context(), id, &service.RevealWindsurfAccountPasswordInput{
		ActorID: subject.UserID,
		IsAdmin: role == service.RoleAdmin,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"password": password})
}

func (h *WindsurfAccountHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	role, _ := middleware2.GetUserRoleFromContext(c)
	if role != service.RoleAdmin {
		response.Forbidden(c, "Only admin can delete windsurf account")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid windsurf account ID")
		return
	}

	if err := h.service.Delete(c.Request.Context(), id, &service.DeleteWindsurfAccountInput{
		ActorID: subject.UserID,
		IsAdmin: true,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, nil)
}
