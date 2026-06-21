package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

type workbenchHandlerAPIKeyStub struct{}

func (s *workbenchHandlerAPIKeyStub) GetByID(_ context.Context, id int64) (*service.APIKey, error) {
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
