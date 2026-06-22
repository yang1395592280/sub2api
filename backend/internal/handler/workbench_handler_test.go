package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type workbenchHandlerRepoStub struct {
	nextConversationID int64
	nextMessageID      int64
	conversations      map[int64]service.WorkbenchConversation
	messages           map[int64][]service.WorkbenchMessage
}

func newWorkbenchHandlerRepoStub() *workbenchHandlerRepoStub {
	return &workbenchHandlerRepoStub{
		nextConversationID: 1,
		nextMessageID:      1,
		conversations:      map[int64]service.WorkbenchConversation{},
		messages:           map[int64][]service.WorkbenchMessage{},
	}
}

func (r *workbenchHandlerRepoStub) CreateConversation(_ context.Context, c *service.WorkbenchConversation) error {
	c.ID = r.nextConversationID
	r.nextConversationID++
	c.CreatedAt = time.Now().UTC()
	c.UpdatedAt = c.CreatedAt
	r.conversations[c.ID] = *c
	return nil
}

func (r *workbenchHandlerRepoStub) ListConversations(_ context.Context, userID int64, params pagination.PaginationParams, filters service.WorkbenchConversationFilters) ([]service.WorkbenchConversation, *pagination.PaginationResult, error) {
	items := make([]service.WorkbenchConversation, 0, len(r.conversations))
	for _, conv := range r.conversations {
		if conv.UserID != userID {
			continue
		}
		if filters.Mode != "" && conv.Mode != filters.Mode {
			continue
		}
		items = append(items, conv)
	}
	return items, &pagination.PaginationResult{
		Total:    int64(len(items)),
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    1,
	}, nil
}

func (r *workbenchHandlerRepoStub) GetConversation(_ context.Context, userID, conversationID int64) (*service.WorkbenchConversation, error) {
	conv, ok := r.conversations[conversationID]
	if !ok || conv.UserID != userID {
		return nil, service.ErrWorkbenchConversationNotFound
	}
	cp := conv
	return &cp, nil
}

func (r *workbenchHandlerRepoStub) SoftDeleteConversation(_ context.Context, userID, conversationID int64) error {
	conv, ok := r.conversations[conversationID]
	if !ok || conv.UserID != userID {
		return service.ErrWorkbenchConversationNotFound
	}
	delete(r.conversations, conv.ID)
	delete(r.messages, conv.ID)
	return nil
}

func (r *workbenchHandlerRepoStub) CreateMessage(_ context.Context, m *service.WorkbenchMessage) error {
	m.ID = r.nextMessageID
	r.nextMessageID++
	m.CreatedAt = time.Now().UTC()
	m.UpdatedAt = m.CreatedAt
	r.messages[m.ConversationID] = append(r.messages[m.ConversationID], *m)
	return nil
}

func (r *workbenchHandlerRepoStub) ListMessages(_ context.Context, userID, conversationID int64) ([]service.WorkbenchMessage, error) {
	conv, ok := r.conversations[conversationID]
	if !ok || conv.UserID != userID {
		return nil, service.ErrWorkbenchConversationNotFound
	}
	return append([]service.WorkbenchMessage(nil), r.messages[conversationID]...), nil
}

func (r *workbenchHandlerRepoStub) ListRecentChatMessages(ctx context.Context, userID, conversationID int64, limit int) ([]service.WorkbenchMessage, error) {
	items, err := r.ListMessages(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(items) > limit {
		return append([]service.WorkbenchMessage(nil), items[len(items)-limit:]...), nil
	}
	return items, nil
}

func (r *workbenchHandlerRepoStub) UpdateMessageAfterGateway(_ context.Context, update service.WorkbenchMessageUpdate) error {
	items := r.messages[update.ConversationID]
	for i, item := range items {
		if item.ID != update.MessageID || item.UserID != update.UserID {
			continue
		}
		item.Content = update.Content
		item.ResponseMetadata = update.ResponseMetadata
		item.ImageOutputs = update.ImageOutputs
		item.Status = update.Status
		item.ErrorMessage = update.ErrorMessage
		item.UpdatedAt = time.Now().UTC()
		items[i] = item
		r.messages[update.ConversationID] = items
		return nil
	}
	return service.ErrWorkbenchConversationNotFound
}

func (r *workbenchHandlerRepoStub) UpdateConversationAfterMessage(_ context.Context, update service.WorkbenchConversationUpdate) error {
	conv, ok := r.conversations[update.ConversationID]
	if !ok || conv.UserID != update.UserID {
		return service.ErrWorkbenchConversationNotFound
	}
	conv.Title = update.Title
	conv.Mode = update.Mode
	conv.APIKeyID = update.APIKeyID
	conv.Endpoint = update.Endpoint
	conv.Model = update.Model
	conv.LastMessagePreview = update.LastMessagePreview
	conv.LastError = update.LastError
	conv.MessageCount += update.MessageCountDelta
	conv.UpdatedAt = time.Now().UTC()
	r.conversations[conv.ID] = conv
	return nil
}

func (r *workbenchHandlerRepoStub) AdminListConversations(_ context.Context, params pagination.PaginationParams, filters service.AdminWorkbenchConversationFilters) ([]service.AdminWorkbenchConversation, *pagination.PaginationResult, error) {
	items := make([]service.AdminWorkbenchConversation, 0, len(r.conversations))
	for _, conv := range r.conversations {
		if filters.Mode != "" && conv.Mode != filters.Mode {
			continue
		}
		if filters.UserID > 0 && conv.UserID != filters.UserID {
			continue
		}
		items = append(items, service.AdminWorkbenchConversation{WorkbenchConversation: conv})
	}
	return items, &pagination.PaginationResult{
		Total:    int64(len(items)),
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    1,
	}, nil
}

func (r *workbenchHandlerRepoStub) AdminGetConversation(_ context.Context, conversationID int64) (*service.AdminWorkbenchConversation, []service.WorkbenchMessage, error) {
	conv, ok := r.conversations[conversationID]
	if !ok {
		return nil, nil, service.ErrWorkbenchConversationNotFound
	}
	detail := service.AdminWorkbenchConversation{WorkbenchConversation: conv}
	return &detail, append([]service.WorkbenchMessage(nil), r.messages[conversationID]...), nil
}

func (r *workbenchHandlerRepoStub) AdminGetStats(context.Context, int) (*service.AdminWorkbenchStats, error) {
	var totalMessages int64
	var imageMessages int64
	var imageBytes int64
	for _, messages := range r.messages {
		for _, message := range messages {
			totalMessages++
			if message.Mode == service.WorkbenchModeImage {
				imageMessages++
			}
			for _, image := range message.ImageOutputs {
				imageBytes += int64(len(image.B64JSON))
			}
		}
	}
	return &service.AdminWorkbenchStats{
		TotalConversations: int64(len(r.conversations)),
		TotalMessages:      totalMessages,
		ImageMessages:      imageMessages,
		ImageBytes:         imageBytes,
		RetentionDays:      7,
	}, nil
}

func (r *workbenchHandlerRepoStub) AdminHardDeleteConversations(_ context.Context, conversationIDs []int64) (int64, error) {
	var deleted int64
	for _, id := range conversationIDs {
		if _, ok := r.conversations[id]; !ok {
			continue
		}
		delete(r.conversations, id)
		delete(r.messages, id)
		deleted++
	}
	return deleted, nil
}

func (r *workbenchHandlerRepoStub) AdminHardDeleteExpiredConversations(_ context.Context, cutoff time.Time) (int64, error) {
	var ids []int64
	for _, conv := range r.conversations {
		if conv.UpdatedAt.Before(cutoff) {
			ids = append(ids, conv.ID)
		}
	}
	return r.AdminHardDeleteConversations(context.Background(), ids)
}

type workbenchHandlerAPIKeyStub struct {
	keys map[int64]*service.APIKey
}

func (s *workbenchHandlerAPIKeyStub) GetByID(_ context.Context, id int64) (*service.APIKey, error) {
	if s.keys != nil {
		key := s.keys[id]
		if key == nil {
			return nil, service.ErrWorkbenchAPIKeyNotFound
		}
		cp := *key
		return &cp, nil
	}
	return &service.APIKey{
		ID:     id,
		UserID: 7,
		Key:    "test-key",
		Status: service.StatusAPIKeyActive,
	}, nil
}

type workbenchHandlerGatewayStub struct{}

func (s *workbenchHandlerGatewayStub) SendChat(_ context.Context, _ string, _ service.WorkbenchGatewayChatRequest) (service.WorkbenchGatewayChatResponse, error) {
	return service.WorkbenchGatewayChatResponse{Content: "assistant reply"}, nil
}

func (s *workbenchHandlerGatewayStub) GenerateImage(_ context.Context, _ string, _ service.WorkbenchGatewayImageRequest) (service.WorkbenchGatewayImageResponse, error) {
	return service.WorkbenchGatewayImageResponse{}, nil
}

type workbenchHandlerGatewayFailStub struct {
	resultContent string
	err           error
}

func (s *workbenchHandlerGatewayFailStub) SendChat(_ context.Context, _ string, _ service.WorkbenchGatewayChatRequest) (service.WorkbenchGatewayChatResponse, error) {
	return service.WorkbenchGatewayChatResponse{Content: s.resultContent}, s.err
}

func (s *workbenchHandlerGatewayFailStub) GenerateImage(_ context.Context, _ string, _ service.WorkbenchGatewayImageRequest) (service.WorkbenchGatewayImageResponse, error) {
	return service.WorkbenchGatewayImageResponse{}, s.err
}

type workbenchHandlerModelProviderStub struct{ models []string }

func (s *workbenchHandlerModelProviderStub) GetAvailableModels(_ context.Context, _ *int64, _ string) []string {
	return append([]string(nil), s.models...)
}

func newWorkbenchAuthedRouter(h *WorkbenchHandler) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		c.Next()
	})
	return r
}

func decodeWorkbenchResponse(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body.Bytes(), &payload))
	return payload
}

func TestWorkbenchHandlerCreateConversationRequiresUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewWorkbenchHandler(service.NewWorkbenchService(newWorkbenchHandlerRepoStub(), &workbenchHandlerAPIKeyStub{}, &workbenchHandlerGatewayStub{}))
	r := gin.New()
	r.POST("/api/v1/workbench/conversations", h.CreateConversation)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/conversations", bytes.NewBufferString(`{"mode":"chat"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWorkbenchHandlerListConversationsReturnsPaginatedData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newWorkbenchHandlerRepoStub()
	svc := service.NewWorkbenchService(repo, &workbenchHandlerAPIKeyStub{}, &workbenchHandlerGatewayStub{})
	_, err := svc.CreateConversation(context.Background(), 7, service.CreateWorkbenchConversationRequest{Mode: service.WorkbenchModeChat, Title: "Hello"})
	require.NoError(t, err)

	h := NewWorkbenchHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		c.Next()
	})
	r.GET("/api/v1/workbench/conversations", h.ListConversations)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/conversations?page=2&page_size=5", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"items"`)
	require.Contains(t, rec.Body.String(), `"total":1`)
	require.Contains(t, rec.Body.String(), `"page":2`)
	require.Contains(t, rec.Body.String(), `"page_size":5`)
}

func TestWorkbenchHandlerListModelsReturnsSelectedAPIKeyModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(11)
	svc := service.NewWorkbenchServiceWithModels(
		newWorkbenchHandlerRepoStub(),
		&workbenchHandlerAPIKeyStub{keys: map[int64]*service.APIKey{
			7: {
				ID:      7,
				UserID:  7,
				Key:     "sk-test",
				Status:  service.StatusAPIKeyActive,
				GroupID: &groupID,
				Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI},
			},
		}},
		&workbenchHandlerGatewayStub{},
		&workbenchHandlerModelProviderStub{models: []string{"gpt-5.5", "gpt-5.4"}},
	)
	h := NewWorkbenchHandler(svc)
	r := newWorkbenchAuthedRouter(h)
	r.GET("/api/v1/workbench/api-keys/:id/models", h.ListModels)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/api-keys/7/models", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, "success", payload["message"])
	data := payload["data"].([]any)
	require.Len(t, data, 2)
	require.Equal(t, "gpt-5.4", data[0].(map[string]any)["name"])
	require.Equal(t, "gpt-5.5", data[1].(map[string]any)["name"])
}

func TestWorkbenchHandlerListModelsInvalidAPIKeyIDReturns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewWorkbenchHandler(service.NewWorkbenchService(newWorkbenchHandlerRepoStub(), &workbenchHandlerAPIKeyStub{}, &workbenchHandlerGatewayStub{}))
	r := newWorkbenchAuthedRouter(h)
	r.GET("/api/v1/workbench/api-keys/:id/models", h.ListModels)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/api-keys/nope/models", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, "Invalid API key ID", payload["message"])
}

func TestWorkbenchHandlerCreateConversationBadJSONReturns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewWorkbenchHandler(service.NewWorkbenchService(newWorkbenchHandlerRepoStub(), &workbenchHandlerAPIKeyStub{}, &workbenchHandlerGatewayStub{}))
	r := newWorkbenchAuthedRouter(h)
	r.POST("/api/v1/workbench/conversations", h.CreateConversation)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/conversations", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, float64(http.StatusBadRequest), payload["code"])
}

func TestWorkbenchHandlerListMessagesInvalidIDReturns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewWorkbenchHandler(service.NewWorkbenchService(newWorkbenchHandlerRepoStub(), &workbenchHandlerAPIKeyStub{}, &workbenchHandlerGatewayStub{}))
	r := newWorkbenchAuthedRouter(h)
	r.GET("/api/v1/workbench/conversations/:id/messages", h.ListMessages)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/conversations/nope/messages", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, "Invalid conversation ID", payload["message"])
}

func TestWorkbenchHandlerDeleteConversationSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newWorkbenchHandlerRepoStub()
	svc := service.NewWorkbenchService(repo, &workbenchHandlerAPIKeyStub{}, &workbenchHandlerGatewayStub{})
	conv, err := svc.CreateConversation(context.Background(), 7, service.CreateWorkbenchConversationRequest{Mode: service.WorkbenchModeChat, Title: "Hello"})
	require.NoError(t, err)

	h := NewWorkbenchHandler(svc)
	r := newWorkbenchAuthedRouter(h)
	r.DELETE("/api/v1/workbench/conversations/:id", h.DeleteConversation)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workbench/conversations/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, "success", payload["message"])
	data := payload["data"].(map[string]any)
	require.Equal(t, "ok", data["message"])
	_, err = repo.GetConversation(context.Background(), 7, conv.ID)
	require.ErrorIs(t, err, service.ErrWorkbenchConversationNotFound)
}

func TestWorkbenchHandlerSendBadJSONReturns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newWorkbenchHandlerRepoStub()
	svc := service.NewWorkbenchService(repo, &workbenchHandlerAPIKeyStub{}, &workbenchHandlerGatewayStub{})
	_, err := svc.CreateConversation(context.Background(), 7, service.CreateWorkbenchConversationRequest{Mode: service.WorkbenchModeChat})
	require.NoError(t, err)

	h := NewWorkbenchHandler(svc)
	r := newWorkbenchAuthedRouter(h)
	r.POST("/api/v1/workbench/conversations/:id/send", h.Send)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/conversations/1/send", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, float64(http.StatusBadRequest), payload["code"])
}

func TestWorkbenchHandlerSendSuccessReturnsResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newWorkbenchHandlerRepoStub()
	svc := service.NewWorkbenchService(repo, &workbenchHandlerAPIKeyStub{}, &workbenchHandlerGatewayStub{})
	_, err := svc.CreateConversation(context.Background(), 7, service.CreateWorkbenchConversationRequest{Mode: service.WorkbenchModeChat})
	require.NoError(t, err)

	h := NewWorkbenchHandler(svc)
	r := newWorkbenchAuthedRouter(h)
	r.POST("/api/v1/workbench/conversations/:id/send", h.Send)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/conversations/1/send", bytes.NewBufferString(`{"mode":"chat","api_key_id":1,"endpoint":"chat_completions","model":"gpt-5.5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, "success", payload["message"])
	data := payload["data"].(map[string]any)
	result := data["result"]
	require.Nil(t, result)
	require.Equal(t, "assistant reply", data["assistant_message"].(map[string]any)["content"])
}

func TestWorkbenchHandlerCreateConversationForeignAPIKeyReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewWorkbenchHandler(service.NewWorkbenchService(
		newWorkbenchHandlerRepoStub(),
		&workbenchHandlerAPIKeyStub{keys: map[int64]*service.APIKey{
			1: {ID: 1, UserID: 99, Key: "sk-other", Status: service.StatusAPIKeyActive},
		}},
		&workbenchHandlerGatewayStub{},
	))
	r := newWorkbenchAuthedRouter(h)
	r.POST("/api/v1/workbench/conversations", h.CreateConversation)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/conversations", bytes.NewBufferString(`{"mode":"chat","api_key_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, service.ErrWorkbenchAPIKeyNotFound.Message, payload["message"])
}

func TestWorkbenchHandlerCreateConversationInactiveAPIKeyReturns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewWorkbenchHandler(service.NewWorkbenchService(
		newWorkbenchHandlerRepoStub(),
		&workbenchHandlerAPIKeyStub{keys: map[int64]*service.APIKey{
			1: {ID: 1, UserID: 7, Key: "sk-test", Status: service.StatusAPIKeyDisabled},
		}},
		&workbenchHandlerGatewayStub{},
	))
	r := newWorkbenchAuthedRouter(h)
	r.POST("/api/v1/workbench/conversations", h.CreateConversation)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/conversations", bytes.NewBufferString(`{"mode":"chat","api_key_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, service.ErrWorkbenchAPIKeyUnavailable.Message, payload["message"])
}

func TestWorkbenchHandlerSendPartialResultReturnsSuccessEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newWorkbenchHandlerRepoStub()
	svc := service.NewWorkbenchService(
		repo,
		&workbenchHandlerAPIKeyStub{},
		&workbenchHandlerGatewayFailStub{
			resultContent: "ignored because error result stores summary",
			err:           errors.New("gateway returned 502: provider failure secret sk-test"),
		},
	)
	_, err := svc.CreateConversation(context.Background(), 7, service.CreateWorkbenchConversationRequest{Mode: service.WorkbenchModeChat})
	require.NoError(t, err)

	h := NewWorkbenchHandler(svc)
	r := newWorkbenchAuthedRouter(h)
	r.POST("/api/v1/workbench/conversations/:id/send", h.Send)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/conversations/1/send", bytes.NewBufferString(`{"mode":"chat","api_key_id":1,"endpoint":"chat_completions","model":"gpt-5.5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, "success", payload["message"])
	data := payload["data"].(map[string]any)
	require.NotNil(t, data["result"])
	errSummary := data["error"].(map[string]any)
	require.Equal(t, "gateway returned 502", errSummary["message"])
	body := rec.Body.String()
	require.NotContains(t, body, "sk-test")
	require.NotContains(t, body, "provider failure")
}

func TestWorkbenchHandlerSendGatewayFailureWithoutResultStillReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewWorkbenchHandler(service.NewWorkbenchService(
		newWorkbenchHandlerRepoStub(),
		&workbenchHandlerAPIKeyStub{},
		&workbenchHandlerGatewayStub{},
	))
	r := newWorkbenchAuthedRouter(h)
	r.POST("/api/v1/workbench/conversations/:id/send", h.Send)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/conversations/999/send", bytes.NewBufferString(`{"mode":"chat","api_key_id":1,"endpoint":"chat_completions","model":"gpt-5.5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	payload := decodeWorkbenchResponse(t, rec.Body)
	require.Equal(t, service.ErrWorkbenchConversationNotFound.Message, payload["message"])
}

func TestWorkbenchHandlerSendPartialResultFallbackMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newWorkbenchHandlerRepoStub()
	svc := service.NewWorkbenchService(
		repo,
		&workbenchHandlerAPIKeyStub{},
		&workbenchHandlerGatewayFailStub{
			resultContent: "",
			err:           infraerrors.InternalServer("WORKBENCH_GATEWAY_FAILED", "raw upstream details secret sk-test"),
		},
	)
	_, err := svc.CreateConversation(context.Background(), 7, service.CreateWorkbenchConversationRequest{Mode: service.WorkbenchModeChat})
	require.NoError(t, err)

	h := NewWorkbenchHandler(svc)
	r := newWorkbenchAuthedRouter(h)
	r.POST("/api/v1/workbench/conversations/:id/send", h.Send)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/conversations/1/send", bytes.NewBufferString(`{"mode":"chat","api_key_id":1,"endpoint":"chat_completions","model":"gpt-5.5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.NotContains(t, body, "sk-test")
	require.NotContains(t, body, "raw upstream details")
}
