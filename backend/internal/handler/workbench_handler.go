package handler

import (
	"net/url"
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

type workbenchSendPartialError struct {
	Message string `json:"message"`
}

type workbenchSendPartialResponse struct {
	Result *service.WorkbenchSendResult `json:"result"`
	Error  any                          `json:"error,omitempty"`
}

func (h *WorkbenchHandler) ListModels(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	apiKeyID, err := parseWorkbenchID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	models, err := h.workbenchService.ListModels(c.Request.Context(), subject.UserID, apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, models)
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

	publicHost, publicScheme := workbenchPublicRequestOrigin(c)
	rewriteWorkbenchImageFileURLs(items, publicHost, publicScheme)
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

	publicHost, publicScheme := workbenchPublicRequestOrigin(c)
	result, err := h.workbenchService.Send(c.Request.Context(), subject.UserID, id, service.WorkbenchSendRequest{
		Mode:         req.Mode,
		APIKeyID:     req.APIKeyID,
		Endpoint:     req.Endpoint,
		Model:        req.Model,
		Input:        req.Input,
		Options:      req.Options,
		PublicHost:   publicHost,
		PublicScheme: publicScheme,
	})
	if err != nil {
		if result != nil {
			response.Success(c, workbenchSendPartialResponse{
				Result: publicWorkbenchSendPartialResult(result),
				Error:  buildWorkbenchSendPartialError(result),
			})
			return
		}
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

func workbenchPublicRequestOrigin(c *gin.Context) (string, string) {
	host := ""
	scheme := "https"
	if c != nil && c.Request != nil {
		host = strings.TrimSpace(c.Request.Host)
		if xfHost := strings.TrimSpace(c.Request.Header.Get("X-Forwarded-Host")); xfHost != "" {
			host = strings.TrimSpace(strings.Split(xfHost, ",")[0])
		}
		if xfProto := strings.TrimSpace(c.Request.Header.Get("X-Forwarded-Proto")); xfProto != "" {
			scheme = strings.TrimSpace(strings.Split(xfProto, ",")[0])
		} else if c.Request.TLS != nil {
			scheme = "https"
		} else if c.Request.URL != nil && strings.TrimSpace(c.Request.URL.Scheme) != "" {
			scheme = strings.TrimSpace(c.Request.URL.Scheme)
		}
	}
	return host, strings.TrimRight(scheme, ":/")
}

func rewriteWorkbenchImageFileURLs(messages []service.WorkbenchMessage, publicHost, publicScheme string) {
	publicHost = strings.TrimSpace(publicHost)
	if publicHost == "" {
		return
	}
	publicScheme = strings.TrimRight(strings.TrimSpace(publicScheme), ":/")
	if publicScheme == "" {
		publicScheme = "https"
	}
	for mi := range messages {
		for ii := range messages[mi].ImageOutputs {
			rawURL := strings.TrimSpace(messages[mi].ImageOutputs[ii].URL)
			if rawURL == "" {
				continue
			}
			parsed, err := url.Parse(rawURL)
			if err != nil || !isInternalWorkbenchImageFileURL(parsed) {
				continue
			}
			rewritten := publicScheme + "://" + publicHost + parsed.EscapedPath()
			if parsed.RawQuery != "" {
				rewritten += "?" + parsed.RawQuery
			}
			messages[mi].ImageOutputs[ii].URL = rewritten
		}
	}
}

func isInternalWorkbenchImageFileURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	path := u.EscapedPath()
	if !strings.HasPrefix(path, "/v1/images/files/") && !strings.HasPrefix(path, "/images/files/") {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	return host == "127.0.0.1" || host == "localhost" || host == "::1" || host == "0.0.0.0"
}

func buildWorkbenchSendPartialError(result *service.WorkbenchSendResult) any {
	if result != nil && result.AssistantMessage.ErrorMessage != nil {
		msg := strings.TrimSpace(*result.AssistantMessage.ErrorMessage)
		if msg != "" {
			return workbenchSendPartialError{Message: summarizeWorkbenchPartialError(msg)}
		}
	}
	return workbenchSendPartialError{Message: "workbench request partially completed"}
}

func publicWorkbenchSendPartialResult(result *service.WorkbenchSendResult) *service.WorkbenchSendResult {
	if result == nil || result.AssistantMessage.ErrorMessage == nil {
		return result
	}
	publicResult := *result
	msg := strings.TrimSpace(*result.AssistantMessage.ErrorMessage)
	if msg == "" {
		return &publicResult
	}
	summary := summarizeWorkbenchPartialError(msg)
	publicResult.AssistantMessage.ErrorMessage = &summary
	publicResult.Conversation.LastError = &summary
	publicResult.Conversation.LastMessagePreview = summary
	return &publicResult
}

func summarizeWorkbenchPartialError(message string) string {
	message = strings.TrimSpace(message)
	if strings.HasPrefix(message, "gateway returned ") {
		if idx := strings.Index(message, ":"); idx > 0 {
			return strings.TrimSpace(message[:idx])
		}
	}
	return message
}

func parseWorkbenchID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}
