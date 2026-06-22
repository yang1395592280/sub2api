package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminWorkbenchRepoStub struct {
	conversations map[int64]service.WorkbenchConversation
	messages      map[int64][]service.WorkbenchMessage
}

func newAdminWorkbenchRepoStub() *adminWorkbenchRepoStub {
	return &adminWorkbenchRepoStub{
		conversations: map[int64]service.WorkbenchConversation{},
		messages:      map[int64][]service.WorkbenchMessage{},
	}
}

func (r *adminWorkbenchRepoStub) CreateConversation(_ context.Context, c *service.WorkbenchConversation) error {
	if c.ID == 0 {
		c.ID = int64(len(r.conversations) + 1)
	}
	r.conversations[c.ID] = *c
	return nil
}

func (r *adminWorkbenchRepoStub) ListConversations(_ context.Context, userID int64, params pagination.PaginationParams, filters service.WorkbenchConversationFilters) ([]service.WorkbenchConversation, *pagination.PaginationResult, error) {
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
	return items, &pagination.PaginationResult{Total: int64(len(items)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *adminWorkbenchRepoStub) GetConversation(_ context.Context, userID, conversationID int64) (*service.WorkbenchConversation, error) {
	conv, ok := r.conversations[conversationID]
	if !ok || conv.UserID != userID {
		return nil, service.ErrWorkbenchConversationNotFound
	}
	cp := conv
	return &cp, nil
}

func (r *adminWorkbenchRepoStub) SoftDeleteConversation(_ context.Context, userID, conversationID int64) error {
	conv, ok := r.conversations[conversationID]
	if !ok || conv.UserID != userID {
		return service.ErrWorkbenchConversationNotFound
	}
	delete(r.conversations, conversationID)
	delete(r.messages, conversationID)
	return nil
}

func (r *adminWorkbenchRepoStub) CreateMessage(_ context.Context, m *service.WorkbenchMessage) error {
	if m.ID == 0 {
		m.ID = int64(len(r.messages[m.ConversationID]) + 1)
	}
	r.messages[m.ConversationID] = append(r.messages[m.ConversationID], *m)
	return nil
}

func (r *adminWorkbenchRepoStub) ListMessages(_ context.Context, userID, conversationID int64) ([]service.WorkbenchMessage, error) {
	conv, ok := r.conversations[conversationID]
	if !ok || conv.UserID != userID {
		return nil, service.ErrWorkbenchConversationNotFound
	}
	return append([]service.WorkbenchMessage(nil), r.messages[conversationID]...), nil
}

func (r *adminWorkbenchRepoStub) ListRecentChatMessages(ctx context.Context, userID, conversationID int64, limit int) ([]service.WorkbenchMessage, error) {
	items, err := r.ListMessages(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(items) > limit {
		return append([]service.WorkbenchMessage(nil), items[len(items)-limit:]...), nil
	}
	return items, nil
}

func (r *adminWorkbenchRepoStub) UpdateMessageAfterGateway(_ context.Context, update service.WorkbenchMessageUpdate) error {
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
		items[i] = item
		r.messages[update.ConversationID] = items
		return nil
	}
	return service.ErrWorkbenchConversationNotFound
}

func (r *adminWorkbenchRepoStub) UpdateConversationAfterMessage(_ context.Context, update service.WorkbenchConversationUpdate) error {
	conv, ok := r.conversations[update.ConversationID]
	if !ok || conv.UserID != update.UserID {
		return service.ErrWorkbenchConversationNotFound
	}
	conv.Mode = update.Mode
	conv.APIKeyID = update.APIKeyID
	conv.Endpoint = update.Endpoint
	conv.Model = update.Model
	conv.LastMessagePreview = update.LastMessagePreview
	conv.LastError = update.LastError
	conv.MessageCount += update.MessageCountDelta
	r.conversations[conv.ID] = conv
	return nil
}

func (r *adminWorkbenchRepoStub) AdminListConversations(_ context.Context, params pagination.PaginationParams, filters service.AdminWorkbenchConversationFilters) ([]service.AdminWorkbenchConversation, *pagination.PaginationResult, error) {
	items := make([]service.AdminWorkbenchConversation, 0, len(r.conversations))
	for _, conv := range r.conversations {
		if filters.Mode != "" && conv.Mode != filters.Mode {
			continue
		}
		if filters.UserID > 0 && conv.UserID != filters.UserID {
			continue
		}
		if filters.HasImages != nil && *filters.HasImages && r.imageCount(conv.ID) == 0 {
			continue
		}
		items = append(items, service.AdminWorkbenchConversation{
			WorkbenchConversation: conv,
			ImageCount:            r.imageCount(conv.ID),
			ImageBytes:            r.imageBytes(conv.ID),
		})
	}
	return items, &pagination.PaginationResult{Total: int64(len(items)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *adminWorkbenchRepoStub) AdminGetConversation(_ context.Context, conversationID int64) (*service.AdminWorkbenchConversation, []service.WorkbenchMessage, error) {
	conv, ok := r.conversations[conversationID]
	if !ok {
		return nil, nil, service.ErrWorkbenchConversationNotFound
	}
	detail := service.AdminWorkbenchConversation{WorkbenchConversation: conv, ImageCount: r.imageCount(conversationID), ImageBytes: r.imageBytes(conversationID)}
	return &detail, append([]service.WorkbenchMessage(nil), r.messages[conversationID]...), nil
}

func (r *adminWorkbenchRepoStub) AdminGetStats(context.Context, int) (*service.AdminWorkbenchStats, error) {
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

func (r *adminWorkbenchRepoStub) AdminHardDeleteConversations(_ context.Context, conversationIDs []int64) (int64, error) {
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

func (r *adminWorkbenchRepoStub) AdminHardDeleteExpiredConversations(_ context.Context, cutoff time.Time) (int64, error) {
	var ids []int64
	for _, conv := range r.conversations {
		if conv.UpdatedAt.Before(cutoff) {
			ids = append(ids, conv.ID)
		}
	}
	return r.AdminHardDeleteConversations(context.Background(), ids)
}

func (r *adminWorkbenchRepoStub) imageCount(conversationID int64) int {
	count := 0
	for _, message := range r.messages[conversationID] {
		for _, image := range message.ImageOutputs {
			if image.URL != "" || image.B64JSON != "" {
				count++
			}
		}
	}
	return count
}

func (r *adminWorkbenchRepoStub) imageBytes(conversationID int64) int64 {
	var size int64
	for _, message := range r.messages[conversationID] {
		for _, image := range message.ImageOutputs {
			size += int64(len(image.B64JSON))
		}
	}
	return size
}

type adminWorkbenchAPIKeyStub struct{}

func (s adminWorkbenchAPIKeyStub) GetByID(context.Context, int64) (*service.APIKey, error) {
	return &service.APIKey{ID: 1, UserID: 7, Key: "sk-test", Status: service.StatusAPIKeyActive}, nil
}

type adminWorkbenchGatewayStub struct{}

func (s adminWorkbenchGatewayStub) SendChat(context.Context, string, service.WorkbenchGatewayChatRequest) (service.WorkbenchGatewayChatResponse, error) {
	return service.WorkbenchGatewayChatResponse{}, nil
}

func (s adminWorkbenchGatewayStub) GenerateImage(context.Context, string, service.WorkbenchGatewayImageRequest) (service.WorkbenchGatewayImageResponse, error) {
	return service.WorkbenchGatewayImageResponse{}, nil
}

func setupAdminWorkbenchRouter(svc *service.WorkbenchService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewWorkbenchHandler(svc)
	router.GET("/api/v1/admin/workbench/stats", handler.GetStats)
	router.GET("/api/v1/admin/workbench/conversations", handler.ListConversations)
	router.GET("/api/v1/admin/workbench/conversations/:id", handler.GetConversation)
	router.POST("/api/v1/admin/workbench/conversations/batch-delete", handler.BatchDeleteConversations)
	router.POST("/api/v1/admin/workbench/conversations/cleanup-expired", handler.CleanupExpiredConversations)
	return router
}

func TestAdminWorkbenchHandlerListsConversationsWithFilters(t *testing.T) {
	repo := newAdminWorkbenchRepoStub()
	chat := service.WorkbenchConversation{
		ID:                 1,
		UserID:             7,
		Title:              "chat",
		Mode:               service.WorkbenchModeChat,
		Endpoint:           service.WorkbenchEndpointChatCompletions,
		Model:              "gpt-5.5",
		LastMessagePreview: "hello",
		MessageCount:       2,
		UpdatedAt:          time.Now().UTC(),
	}
	image := service.WorkbenchConversation{
		ID:                 2,
		UserID:             8,
		Title:              "image",
		Mode:               service.WorkbenchModeImage,
		Endpoint:           service.WorkbenchEndpointImagesGenerations,
		Model:              "gpt-image-2",
		LastMessagePreview: "draw",
		MessageCount:       1,
		UpdatedAt:          time.Now().UTC(),
	}
	repo.conversations[chat.ID] = chat
	repo.conversations[image.ID] = image
	repo.messages[image.ID] = []service.WorkbenchMessage{{
		ID:             20,
		UserID:         8,
		ConversationID: image.ID,
		Mode:           service.WorkbenchModeImage,
		Role:           service.WorkbenchRoleAssistant,
		Status:         service.WorkbenchMessageStatusSuccess,
		ImageOutputs: []service.WorkbenchImageOutput{{
			B64JSON:  "ZmFrZQ==",
			MimeType: "image/png",
		}},
	}}

	router := setupAdminWorkbenchRouter(service.NewWorkbenchService(repo, adminWorkbenchAPIKeyStub{}, adminWorkbenchGatewayStub{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/workbench/conversations?page=1&page_size=20&mode=image&user_id=8&has_images=true", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Data struct {
			Items []struct {
				ID         int64  `json:"id"`
				UserID     int64  `json:"user_id"`
				Mode       string `json:"mode"`
				ImageCount int    `json:"image_count"`
				ImageBytes int64  `json:"image_bytes"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, int64(1), payload.Data.Total)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, int64(2), payload.Data.Items[0].ID)
	require.Equal(t, int64(8), payload.Data.Items[0].UserID)
	require.Equal(t, service.WorkbenchModeImage, payload.Data.Items[0].Mode)
	require.Equal(t, 1, payload.Data.Items[0].ImageCount)
	require.Greater(t, payload.Data.Items[0].ImageBytes, int64(0))
}

func TestAdminWorkbenchHandlerGetsConversationDetail(t *testing.T) {
	repo := newAdminWorkbenchRepoStub()
	conv := service.WorkbenchConversation{
		ID:       9,
		UserID:   7,
		Title:    "detail",
		Mode:     service.WorkbenchModeChat,
		Endpoint: service.WorkbenchEndpointChatCompletions,
		Model:    "gpt-5.5",
	}
	repo.conversations[conv.ID] = conv
	repo.messages[conv.ID] = []service.WorkbenchMessage{{
		ID:             90,
		UserID:         7,
		ConversationID: conv.ID,
		Mode:           service.WorkbenchModeChat,
		Role:           service.WorkbenchRoleUser,
		Content:        "hello",
		Status:         service.WorkbenchMessageStatusSuccess,
	}}
	router := setupAdminWorkbenchRouter(service.NewWorkbenchService(repo, adminWorkbenchAPIKeyStub{}, adminWorkbenchGatewayStub{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/workbench/conversations/9", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Data struct {
			Conversation struct {
				ID int64 `json:"id"`
			} `json:"conversation"`
			Messages []struct {
				ID      int64  `json:"id"`
				Content string `json:"content"`
			} `json:"messages"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, int64(9), payload.Data.Conversation.ID)
	require.Len(t, payload.Data.Messages, 1)
	require.Equal(t, "hello", payload.Data.Messages[0].Content)
}

func TestAdminWorkbenchHandlerDeletesAndCleansExpiredConversations(t *testing.T) {
	repo := newAdminWorkbenchRepoStub()
	oldTime := time.Now().UTC().AddDate(0, 0, -8)
	repo.conversations[1] = service.WorkbenchConversation{ID: 1, UserID: 7, Mode: service.WorkbenchModeChat, UpdatedAt: time.Now().UTC()}
	repo.conversations[2] = service.WorkbenchConversation{ID: 2, UserID: 8, Mode: service.WorkbenchModeImage, UpdatedAt: oldTime}
	repo.conversations[3] = service.WorkbenchConversation{ID: 3, UserID: 8, Mode: service.WorkbenchModeChat, UpdatedAt: oldTime}
	router := setupAdminWorkbenchRouter(service.NewWorkbenchService(repo, adminWorkbenchAPIKeyStub{}, adminWorkbenchGatewayStub{}))

	deleteReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/workbench/conversations/batch-delete", bytes.NewBufferString(`{"conversation_ids":[1,1,0]}`))
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)

	require.Equal(t, http.StatusOK, deleteRec.Code)
	require.NotContains(t, repo.conversations, int64(1))

	cleanupReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/workbench/conversations/cleanup-expired", bytes.NewBufferString(`{"retention_days":7}`))
	cleanupReq.Header.Set("Content-Type", "application/json")
	cleanupRec := httptest.NewRecorder()
	router.ServeHTTP(cleanupRec, cleanupReq)

	require.Equal(t, http.StatusOK, cleanupRec.Code)
	var payload struct {
		Data struct {
			Deleted int64 `json:"deleted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(cleanupRec.Body.Bytes(), &payload))
	require.Equal(t, int64(2), payload.Data.Deleted)
	require.Empty(t, repo.conversations)
}
