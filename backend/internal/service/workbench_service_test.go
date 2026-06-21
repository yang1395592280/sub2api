package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestWorkbenchServiceSendChatPersistsUserAndAssistantMessages(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	gateway := &workbenchGatewayStub{chat: WorkbenchGatewayChatResponse{
		Content:  "你好，我可以帮你。",
		Metadata: map[string]any{"request_id": "chatcmpl_1"},
	}}
	svc := NewWorkbenchService(repo, apiKeys, gateway)

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{Mode: WorkbenchModeChat})
	require.NoError(t, err)

	result, err := svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointChatCompletions,
		Model:    "gpt-5.5",
		Input:    "你好",
	})
	require.NoError(t, err)
	require.Equal(t, WorkbenchMessageStatusSuccess, result.AssistantMessage.Status)
	require.Equal(t, "你好，我可以帮你。", result.AssistantMessage.Content)
	require.Len(t, repo.messages[conv.ID], 2)
	require.Equal(t, "Bearer sk-test", gateway.lastAuthorization)
	require.Equal(t, "gpt-5.5", gateway.lastChat.Model)
	require.Len(t, gateway.lastChat.Messages, 1)
}

func TestWorkbenchServiceSendRejectsForeignAPIKey(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 99, Key: "sk-other", Status: StatusAPIKeyActive, Name: "other"},
	}}
	svc := NewWorkbenchService(repo, apiKeys, &workbenchGatewayStub{})

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{Mode: WorkbenchModeChat})
	require.NoError(t, err)

	_, err = svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointChatCompletions,
		Model:    "gpt-5.5",
		Input:    "hi",
	})
	require.ErrorIs(t, err, ErrWorkbenchAPIKeyNotFound)
	require.Empty(t, repo.messages[conv.ID])
}

func TestWorkbenchServiceSendImagePersistsImageOutputs(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	gateway := &workbenchGatewayStub{image: WorkbenchGatewayImageResponse{
		Images:   []WorkbenchImageOutput{{URL: "https://img.example/1.png", MimeType: "image/png"}},
		Metadata: map[string]any{"image_count": float64(1)},
	}}
	svc := NewWorkbenchService(repo, apiKeys, gateway)

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{Mode: WorkbenchModeImage})
	require.NoError(t, err)

	result, err := svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeImage,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointImagesGenerations,
		Model:    "gpt-image-2",
		Input:    "一张青绿色的工作台插画",
		Options:  map[string]any{"size": "1024x1024", "n": float64(1)},
	})
	require.NoError(t, err)
	require.Len(t, result.AssistantMessage.ImageOutputs, 1)
	require.Equal(t, "https://img.example/1.png", result.AssistantMessage.ImageOutputs[0].URL)
	require.Equal(t, "1024x1024", gateway.lastImage.Options["size"])
}

func TestWorkbenchServiceSendStoresErrorMessageWhenGatewayFails(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	gateway := &workbenchGatewayStub{err: errors.New("upstream timeout with secret sk-test")}
	svc := NewWorkbenchService(repo, apiKeys, gateway)

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{Mode: WorkbenchModeChat})
	require.NoError(t, err)

	result, err := svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointChatCompletions,
		Model:    "gpt-5.5",
		Input:    "hi",
	})
	require.Error(t, err)
	require.Equal(t, WorkbenchMessageStatusError, result.AssistantMessage.Status)
	require.NotContains(t, *result.AssistantMessage.ErrorMessage, "sk-test")
	require.Len(t, repo.messages[conv.ID], 2)
}

type workbenchGatewayStub struct {
	chat              WorkbenchGatewayChatResponse
	image             WorkbenchGatewayImageResponse
	err               error
	lastAuthorization string
	lastChat          WorkbenchGatewayChatRequest
	lastImage         WorkbenchGatewayImageRequest
}

func (g *workbenchGatewayStub) SendChat(_ context.Context, authorization string, req WorkbenchGatewayChatRequest) (WorkbenchGatewayChatResponse, error) {
	g.lastAuthorization = authorization
	g.lastChat = req
	return g.chat, g.err
}

func (g *workbenchGatewayStub) GenerateImage(_ context.Context, authorization string, req WorkbenchGatewayImageRequest) (WorkbenchGatewayImageResponse, error) {
	g.lastAuthorization = authorization
	g.lastImage = req
	return g.image, g.err
}

type workbenchAPIKeyLookupStub struct{ keys map[int64]*APIKey }

func (s *workbenchAPIKeyLookupStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	key := s.keys[id]
	if key == nil {
		return nil, ErrWorkbenchAPIKeyNotFound
	}
	cp := *key
	return &cp, nil
}

type workbenchMemoryRepo struct {
	nextConversationID int64
	nextMessageID      int64
	conversations      map[int64]WorkbenchConversation
	messages           map[int64][]WorkbenchMessage
}

func newWorkbenchMemoryRepo() *workbenchMemoryRepo {
	return &workbenchMemoryRepo{
		nextConversationID: 1,
		nextMessageID:      1,
		conversations:      map[int64]WorkbenchConversation{},
		messages:           map[int64][]WorkbenchMessage{},
	}
}

func (r *workbenchMemoryRepo) CreateConversation(_ context.Context, c *WorkbenchConversation) error {
	c.ID = r.nextConversationID
	r.nextConversationID++
	c.CreatedAt = time.Now().UTC()
	c.UpdatedAt = c.CreatedAt
	r.conversations[c.ID] = *c
	return nil
}

func (r *workbenchMemoryRepo) ListConversations(_ context.Context, userID int64, params pagination.PaginationParams, filters WorkbenchConversationFilters) ([]WorkbenchConversation, *pagination.PaginationResult, error) {
	out := []WorkbenchConversation{}
	for _, c := range r.conversations {
		if c.UserID != userID {
			continue
		}
		if filters.Mode != "" && c.Mode != filters.Mode {
			continue
		}
		out = append(out, c)
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *workbenchMemoryRepo) GetConversation(_ context.Context, userID, conversationID int64) (*WorkbenchConversation, error) {
	c, ok := r.conversations[conversationID]
	if !ok || c.UserID != userID {
		return nil, ErrWorkbenchConversationNotFound
	}
	cp := c
	return &cp, nil
}

func (r *workbenchMemoryRepo) SoftDeleteConversation(_ context.Context, userID, conversationID int64) error {
	c, ok := r.conversations[conversationID]
	if !ok || c.UserID != userID {
		return ErrWorkbenchConversationNotFound
	}
	delete(r.conversations, conversationID)
	delete(r.messages, conversationID)
	return nil
}

func (r *workbenchMemoryRepo) CreateMessage(_ context.Context, m *WorkbenchMessage) error {
	m.ID = r.nextMessageID
	r.nextMessageID++
	m.CreatedAt = time.Now().UTC()
	m.UpdatedAt = m.CreatedAt
	r.messages[m.ConversationID] = append(r.messages[m.ConversationID], *m)
	return nil
}

func (r *workbenchMemoryRepo) ListMessages(_ context.Context, userID, conversationID int64) ([]WorkbenchMessage, error) {
	c, ok := r.conversations[conversationID]
	if !ok || c.UserID != userID {
		return nil, ErrWorkbenchConversationNotFound
	}
	out := []WorkbenchMessage{}
	for _, m := range r.messages[conversationID] {
		if m.UserID == userID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *workbenchMemoryRepo) ListRecentChatMessages(ctx context.Context, userID, conversationID int64, limit int) ([]WorkbenchMessage, error) {
	messages, err := r.ListMessages(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	filtered := []WorkbenchMessage{}
	for _, m := range messages {
		if m.Mode == WorkbenchModeChat && m.Status == WorkbenchMessageStatusSuccess {
			filtered = append(filtered, m)
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered, nil
}

func (r *workbenchMemoryRepo) UpdateConversationAfterMessage(_ context.Context, update WorkbenchConversationUpdate) error {
	c, ok := r.conversations[update.ConversationID]
	if !ok || c.UserID != update.UserID {
		return ErrWorkbenchConversationNotFound
	}
	c.Mode = update.Mode
	c.APIKeyID = update.APIKeyID
	c.Endpoint = update.Endpoint
	c.Model = update.Model
	if update.Title != "" {
		c.Title = update.Title
	}
	c.LastMessagePreview = update.LastMessagePreview
	c.LastError = update.LastError
	c.MessageCount += update.MessageCountDelta
	c.UpdatedAt = time.Now().UTC()
	r.conversations[c.ID] = c
	return nil
}
