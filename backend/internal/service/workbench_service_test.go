package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	svc.asyncRunner = func(fn func()) { fn() }

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
	require.Equal(t, WorkbenchMessageStatusPending, result.AssistantMessage.Status)
	messages, err := repo.ListMessages(ctx, 42, conv.ID)
	require.NoError(t, err)
	require.Len(t, messages[1].ImageOutputs, 1)
	require.Equal(t, "https://img.example/1.png", messages[1].ImageOutputs[0].URL)
	require.Equal(t, "1024x1024", gateway.lastImage.Options["size"])
}

func TestWorkbenchServiceSendImageReturnsPendingBeforeGatewayCompletes(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	gateway := newWorkbenchBlockingImageGateway(WorkbenchGatewayImageResponse{
		Images:   []WorkbenchImageOutput{{URL: "https://img.example/async.png", MimeType: "image/png"}},
		Metadata: map[string]any{"image_count": float64(1)},
	})
	svc := NewWorkbenchService(repo, apiKeys, gateway)

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{Mode: WorkbenchModeImage})
	require.NoError(t, err)

	done := make(chan struct {
		result *WorkbenchSendResult
		err    error
	}, 1)
	go func() {
		result, err := svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
			Mode:     WorkbenchModeImage,
			APIKeyID: 7,
			Endpoint: WorkbenchEndpointImagesGenerations,
			Model:    "gpt-image-2",
			Input:    "draw async image",
		})
		done <- struct {
			result *WorkbenchSendResult
			err    error
		}{result: result, err: err}
	}()

	var initial struct {
		result *WorkbenchSendResult
		err    error
	}
	select {
	case initial = <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected image send to return pending before gateway completes")
	}
	require.NoError(t, initial.err)
	require.Equal(t, WorkbenchMessageStatusPending, initial.result.AssistantMessage.Status)
	require.Empty(t, initial.result.AssistantMessage.ImageOutputs)
	require.Equal(t, "生图任务已提交，正在生成图片。", initial.result.AssistantMessage.Content)

	gateway.release()
	require.Eventually(t, func() bool {
		messages, err := repo.ListMessages(ctx, 42, conv.ID)
		if err != nil || len(messages) != 2 {
			return false
		}
		return messages[1].Status == WorkbenchMessageStatusSuccess &&
			len(messages[1].ImageOutputs) == 1 &&
			messages[1].ImageOutputs[0].URL == "https://img.example/async.png"
	}, time.Second, 10*time.Millisecond)
}

func TestWorkbenchServiceSendImageDefaultsNonImageModel(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	gateway := &workbenchGatewayStub{image: WorkbenchGatewayImageResponse{
		Images: []WorkbenchImageOutput{{B64JSON: "ZmFrZQ==", MimeType: "image/png"}},
	}}
	svc := NewWorkbenchService(repo, apiKeys, gateway)
	svc.asyncRunner = func(fn func()) { fn() }

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{Mode: WorkbenchModeImage})
	require.NoError(t, err)

	result, err := svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeImage,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointImagesGenerations,
		Model:    "gpt-5.5",
		Input:    "draw a cat",
	})

	require.NoError(t, err)
	require.Equal(t, "gpt-image-2", gateway.lastImage.Model)
	require.Equal(t, "gpt-image-2", result.UserMessage.Model)
	require.Equal(t, "gpt-image-2", result.AssistantMessage.Model)
	require.Equal(t, "gpt-image-2", result.Conversation.Model)
}

func TestWorkbenchServiceSendImageAsyncFailureUpdatesPendingMessage(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	gateway := &workbenchGatewayStub{err: errors.New("gateway returned 502: provider failure secret sk-test")}
	svc := NewWorkbenchService(repo, apiKeys, gateway)
	svc.asyncRunner = func(fn func()) { fn() }

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{Mode: WorkbenchModeImage})
	require.NoError(t, err)

	result, err := svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeImage,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointImagesGenerations,
		Model:    "gpt-image-2",
		Input:    "draw a cat",
	})

	require.NoError(t, err)
	require.Equal(t, WorkbenchMessageStatusPending, result.AssistantMessage.Status)
	messages, err := repo.ListMessages(ctx, 42, conv.ID)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	require.Equal(t, WorkbenchMessageStatusError, messages[1].Status)
	require.NotNil(t, messages[1].ErrorMessage)
	require.Equal(t, "gateway returned 502", *messages[1].ErrorMessage)
	require.NotContains(t, *messages[1].ErrorMessage, "sk-test")
}

func TestWorkbenchServiceSendStoresErrorMessageWhenGatewayFails(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	gateway := &workbenchGatewayStub{err: errors.New("gateway returned 502: provider failure secret sk-test")}
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
	require.Equal(t, "gateway returned 502", *result.AssistantMessage.ErrorMessage)
	require.Len(t, repo.messages[conv.ID], 2)
	require.NotContains(t, *result.AssistantMessage.ErrorMessage, "sk-test")
	require.NotContains(t, *result.AssistantMessage.ErrorMessage, "provider failure")
	require.NotContains(t, repo.messages[conv.ID][1].Content, "provider failure")
}

func TestWorkbenchServiceCreateConversationRejectsInvalidMode(t *testing.T) {
	ctx := context.Background()
	svc := NewWorkbenchService(newWorkbenchMemoryRepo(), &workbenchAPIKeyLookupStub{}, &workbenchGatewayStub{})

	_, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{Mode: "audio"})

	require.ErrorIs(t, err, ErrWorkbenchInvalidMode)
}

func TestWorkbenchServiceCreateConversationRejectsForeignAPIKey(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 99, Key: "sk-other", Status: StatusAPIKeyActive, Name: "other"},
	}}
	svc := NewWorkbenchService(repo, apiKeys, &workbenchGatewayStub{})

	_, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: workbenchInt64Ptr(7),
	})

	require.ErrorIs(t, err, ErrWorkbenchAPIKeyNotFound)
	require.Empty(t, repo.conversations)
}

func TestWorkbenchServiceCreateConversationRejectsInactiveAPIKey(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyDisabled, Name: "main"},
	}}
	svc := NewWorkbenchService(repo, apiKeys, &workbenchGatewayStub{})

	_, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: workbenchInt64Ptr(7),
	})

	require.ErrorIs(t, err, ErrWorkbenchAPIKeyUnavailable)
	require.Empty(t, repo.conversations)
}

func TestWorkbenchServiceCreateConversationStoresOwnerActiveAPIKey(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	svc := NewWorkbenchService(repo, apiKeys, &workbenchGatewayStub{})

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: workbenchInt64Ptr(7),
	})

	require.NoError(t, err)
	require.NotNil(t, conv.APIKeyID)
	require.Equal(t, int64(7), *conv.APIKeyID)
	require.NotEmpty(t, repo.conversations)
	require.NotNil(t, repo.conversations[conv.ID].APIKeyID)
	require.Equal(t, int64(7), *repo.conversations[conv.ID].APIKeyID)
}

func TestWorkbenchServiceListModelsUsesSelectedAPIKeyGroup(t *testing.T) {
	ctx := context.Background()
	groupID := int64(11)
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {
			ID:      7,
			UserID:  42,
			Key:     "sk-test",
			Status:  StatusAPIKeyActive,
			Name:    "main",
			GroupID: &groupID,
			Group:   &Group{ID: groupID, Platform: PlatformOpenAI},
		},
	}}
	provider := &workbenchModelProviderStub{models: []string{"gpt-5.5", "gpt-5.4", "gpt-5.5", " "}}
	svc := NewWorkbenchServiceWithModels(newWorkbenchMemoryRepo(), apiKeys, &workbenchGatewayStub{}, provider)

	models, err := svc.ListModels(ctx, 42, 7)

	require.NoError(t, err)
	require.Equal(t, &groupID, provider.lastGroupID)
	require.Equal(t, PlatformOpenAI, provider.lastPlatform)
	require.Equal(t, []WorkbenchModel{{Name: "gpt-5.4"}, {Name: "gpt-5.5"}}, models)
}

func TestWorkbenchServiceListModelsAppliesCustomModelsList(t *testing.T) {
	ctx := context.Background()
	groupID := int64(12)
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {
			ID:      7,
			UserID:  42,
			Key:     "sk-test",
			Status:  StatusAPIKeyActive,
			Name:    "main",
			GroupID: &groupID,
			Group: &Group{
				ID:       groupID,
				Platform: PlatformOpenAI,
				ModelsListConfig: GroupModelsListConfig{
					Enabled: true,
					Models:  []string{"gpt-5.5", "not-available"},
				},
			},
		},
	}}
	provider := &workbenchModelProviderStub{models: []string{"gpt-5.5", "gpt-5.4"}}
	svc := NewWorkbenchServiceWithModels(newWorkbenchMemoryRepo(), apiKeys, &workbenchGatewayStub{}, provider)

	models, err := svc.ListModels(ctx, 42, 7)

	require.NoError(t, err)
	require.Equal(t, []WorkbenchModel{{Name: "gpt-5.5"}}, models)
}

func TestWorkbenchServiceListModelsRejectsForeignAPIKey(t *testing.T) {
	ctx := context.Background()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 99, Key: "sk-other", Status: StatusAPIKeyActive, Name: "other"},
	}}
	svc := NewWorkbenchServiceWithModels(newWorkbenchMemoryRepo(), apiKeys, &workbenchGatewayStub{}, &workbenchModelProviderStub{})

	_, err := svc.ListModels(ctx, 42, 7)

	require.ErrorIs(t, err, ErrWorkbenchAPIKeyNotFound)
}

func TestWorkbenchServiceSendRejectsInvalidMode(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	svc := NewWorkbenchService(repo, &workbenchAPIKeyLookupStub{}, &workbenchGatewayStub{})

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{})
	require.NoError(t, err)

	_, err = svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     "audio",
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointChatCompletions,
		Model:    "gpt-5.5",
		Input:    "hi",
	})

	require.ErrorIs(t, err, ErrWorkbenchInvalidMode)
}

func TestWorkbenchServiceSendRejectsInvalidEndpoint(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	svc := NewWorkbenchService(repo, &workbenchAPIKeyLookupStub{}, &workbenchGatewayStub{})

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{})
	require.NoError(t, err)

	_, err = svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointImagesGenerations,
		Model:    "gpt-5.5",
		Input:    "hi",
	})

	require.ErrorIs(t, err, ErrWorkbenchInvalidEndpoint)
}

func TestWorkbenchServiceSendRejectsEmptyInput(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	svc := NewWorkbenchService(repo, &workbenchAPIKeyLookupStub{}, &workbenchGatewayStub{})

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{})
	require.NoError(t, err)

	_, err = svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointChatCompletions,
		Model:    "gpt-5.5",
		Input:    " \n\t ",
	})

	require.ErrorIs(t, err, ErrWorkbenchEmptyInput)
}

func TestWorkbenchServiceSendRejectsEmptyModel(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	svc := NewWorkbenchService(repo, apiKeys, &workbenchGatewayStub{})

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{})
	require.NoError(t, err)

	_, err = svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointChatCompletions,
		Model:    " \n\t ",
		Input:    "hi",
	})

	require.ErrorIs(t, err, ErrWorkbenchEmptyModel)
	require.Empty(t, repo.messages[conv.ID])
}

func TestWorkbenchServiceSendRejectsInactiveAPIKey(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyDisabled, Name: "main"},
	}}
	svc := NewWorkbenchService(repo, apiKeys, &workbenchGatewayStub{})

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{})
	require.NoError(t, err)

	_, err = svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointChatCompletions,
		Model:    "gpt-5.5",
		Input:    "hi",
	})

	require.ErrorIs(t, err, ErrWorkbenchAPIKeyUnavailable)
}

func TestWorkbenchServiceSendRejectsConversationOwnershipFailure(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	svc := NewWorkbenchService(repo, apiKeys, &workbenchGatewayStub{})

	conv, err := svc.CreateConversation(ctx, 99, CreateWorkbenchConversationRequest{})
	require.NoError(t, err)

	_, err = svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointChatCompletions,
		Model:    "gpt-5.5",
		Input:    "hi",
	})

	require.ErrorIs(t, err, ErrWorkbenchConversationNotFound)
}

func TestWorkbenchServiceSendChatHistoryIsBoundedAndDoesNotDuplicateCurrentInput(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	gateway := &workbenchGatewayStub{chat: WorkbenchGatewayChatResponse{Content: "done"}}
	svc := NewWorkbenchService(repo, apiKeys, gateway)

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{})
	require.NoError(t, err)

	for i := 0; i < 25; i++ {
		userText := "old-user-" + string(rune('a'+i))
		assistantText := "old-assistant-" + string(rune('a'+i))
		require.NoError(t, repo.CreateMessage(ctx, &WorkbenchMessage{
			ConversationID: conv.ID,
			UserID:         42,
			Mode:           WorkbenchModeChat,
			Role:           WorkbenchRoleUser,
			Content:        userText,
			Status:         WorkbenchMessageStatusSuccess,
		}))
		require.NoError(t, repo.CreateMessage(ctx, &WorkbenchMessage{
			ConversationID: conv.ID,
			UserID:         42,
			Mode:           WorkbenchModeChat,
			Role:           WorkbenchRoleAssistant,
			Content:        assistantText,
			Status:         WorkbenchMessageStatusSuccess,
		}))
	}

	currentInput := "current prompt"
	_, err = svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeChat,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointChatCompletions,
		Model:    "gpt-5.5",
		Input:    currentInput,
	})
	require.NoError(t, err)
	require.Len(t, gateway.lastChat.Messages, workbenchHistoryLimit+1)
	require.Equal(t, currentInput, gateway.lastChat.Messages[len(gateway.lastChat.Messages)-1].Content)

	currentCount := 0
	for _, message := range gateway.lastChat.Messages {
		if message.Content == currentInput {
			currentCount++
		}
	}
	require.Equal(t, 1, currentCount)
	require.Equal(t, "old-user-p", gateway.lastChat.Messages[0].Content)
}

func TestHTTPWorkbenchGatewayClientSendChatIncludesStreamFalse(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer server.Close()

	client := &HTTPWorkbenchGatewayClient{client: server.Client(), baseURL: server.URL}

	resp, err := client.SendChat(context.Background(), "Bearer sk-test", WorkbenchGatewayChatRequest{
		Model:    "gpt-5.5",
		Messages: []WorkbenchGatewayMessage{{Role: WorkbenchRoleUser, Content: "hi"}},
		Options:  map[string]any{"temperature": 0.2},
	})

	require.NoError(t, err)
	require.Equal(t, "hello", resp.Content)
	require.Equal(t, false, got["stream"])
	require.Equal(t, "gpt-5.5", got["model"])
}

func TestNewHTTPWorkbenchGatewayClientUsesLongTimeoutForAsyncImages(t *testing.T) {
	client, ok := NewHTTPWorkbenchGatewayClient(nil).(*HTTPWorkbenchGatewayClient)
	require.True(t, ok)
	require.Equal(t, 5*time.Minute, client.client.Timeout)
}

func TestHTTPWorkbenchGatewayClientPostJSONDoesNotLeakRawBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"provider exploded","secret":"sk-secret","provider":"openai"}}`))
	}))
	defer server.Close()

	client := &HTTPWorkbenchGatewayClient{client: server.Client(), baseURL: server.URL}

	err := client.postJSON(context.Background(), "/v1/chat/completions", "Bearer sk-test", map[string]any{"model": "gpt"}, &map[string]any{})

	require.EqualError(t, err, "gateway returned 502")
	require.NotContains(t, err.Error(), "sk-secret")
	require.NotContains(t, err.Error(), "provider exploded")
	require.NotContains(t, err.Error(), "openai")
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

type workbenchBlockingImageGateway struct {
	resp     WorkbenchGatewayImageResponse
	started  chan struct{}
	releaseC chan struct{}
}

func newWorkbenchBlockingImageGateway(resp WorkbenchGatewayImageResponse) *workbenchBlockingImageGateway {
	return &workbenchBlockingImageGateway{
		resp:     resp,
		started:  make(chan struct{}),
		releaseC: make(chan struct{}),
	}
}

func (g *workbenchBlockingImageGateway) SendChat(context.Context, string, WorkbenchGatewayChatRequest) (WorkbenchGatewayChatResponse, error) {
	return WorkbenchGatewayChatResponse{}, nil
}

func (g *workbenchBlockingImageGateway) GenerateImage(context.Context, string, WorkbenchGatewayImageRequest) (WorkbenchGatewayImageResponse, error) {
	close(g.started)
	<-g.releaseC
	return g.resp, nil
}

func (g *workbenchBlockingImageGateway) release() {
	<-g.started
	close(g.releaseC)
}

type workbenchModelProviderStub struct {
	models       []string
	lastGroupID  *int64
	lastPlatform string
}

func (p *workbenchModelProviderStub) GetAvailableModels(_ context.Context, groupID *int64, platform string) []string {
	p.lastGroupID = groupID
	p.lastPlatform = platform
	return append([]string(nil), p.models...)
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

func (r *workbenchMemoryRepo) UpdateMessageAfterGateway(_ context.Context, update WorkbenchMessageUpdate) error {
	messages := r.messages[update.ConversationID]
	for i, m := range messages {
		if m.ID != update.MessageID || m.UserID != update.UserID {
			continue
		}
		m.Content = update.Content
		m.ResponseMetadata = nonNilWorkbenchMap(update.ResponseMetadata)
		m.ImageOutputs = update.ImageOutputs
		m.Status = update.Status
		m.ErrorMessage = update.ErrorMessage
		m.UpdatedAt = time.Now().UTC()
		messages[i] = m
		r.messages[update.ConversationID] = messages
		return nil
	}
	return ErrWorkbenchConversationNotFound
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

func workbenchInt64Ptr(v int64) *int64 {
	return &v
}
