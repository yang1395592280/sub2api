package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	workbenchHistoryLimit           = 20
	workbenchConversationTitleMax   = 160
	workbenchConversationPreviewMax = 120
	workbenchMessageContentMax      = 12000
	workbenchErrorMessageMax        = 500
)

type WorkbenchService struct {
	repo    WorkbenchRepository
	apiKeys WorkbenchAPIKeyLookup
	gateway WorkbenchGatewayClient
}

func NewWorkbenchService(repo WorkbenchRepository, apiKeys WorkbenchAPIKeyLookup, gateway WorkbenchGatewayClient) *WorkbenchService {
	return &WorkbenchService{repo: repo, apiKeys: apiKeys, gateway: gateway}
}

func (s *WorkbenchService) ListConversations(ctx context.Context, userID int64, params pagination.PaginationParams, filters WorkbenchConversationFilters) ([]WorkbenchConversation, *pagination.PaginationResult, error) {
	if filters.Mode != "" && filters.Mode != WorkbenchModeChat && filters.Mode != WorkbenchModeImage {
		return nil, nil, ErrWorkbenchInvalidMode
	}
	return s.repo.ListConversations(ctx, userID, params, filters)
}

func (s *WorkbenchService) CreateConversation(ctx context.Context, userID int64, req CreateWorkbenchConversationRequest) (*WorkbenchConversation, error) {
	mode := normalizeWorkbenchMode(req.Mode)
	endpoint := normalizeWorkbenchEndpoint(req.Endpoint, mode)
	if err := validateWorkbenchModeEndpoint(mode, endpoint); err != nil {
		return nil, err
	}

	title := truncateWorkbenchText(strings.TrimSpace(req.Title), workbenchConversationTitleMax)
	if title == "" {
		if mode == WorkbenchModeImage {
			title = "新生图会话"
		} else {
			title = "新对话"
		}
	}

	conv := &WorkbenchConversation{
		UserID:   userID,
		Title:    title,
		Mode:     mode,
		APIKeyID: req.APIKeyID,
		Endpoint: endpoint,
		Model:    strings.TrimSpace(req.Model),
	}
	if err := s.repo.CreateConversation(ctx, conv); err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *WorkbenchService) ListMessages(ctx context.Context, userID, conversationID int64) ([]WorkbenchMessage, error) {
	if _, err := s.repo.GetConversation(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	return s.repo.ListMessages(ctx, userID, conversationID)
}

func (s *WorkbenchService) DeleteConversation(ctx context.Context, userID, conversationID int64) error {
	return s.repo.SoftDeleteConversation(ctx, userID, conversationID)
}

func (s *WorkbenchService) Send(ctx context.Context, userID, conversationID int64, req WorkbenchSendRequest) (*WorkbenchSendResult, error) {
	mode := normalizeWorkbenchMode(req.Mode)
	endpoint := normalizeWorkbenchEndpoint(req.Endpoint, mode)
	if err := validateWorkbenchModeEndpoint(mode, endpoint); err != nil {
		return nil, err
	}

	input := truncateWorkbenchText(strings.TrimSpace(req.Input), workbenchMessageContentMax)
	if input == "" {
		return nil, ErrWorkbenchEmptyInput
	}

	conv, err := s.repo.GetConversation(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}

	apiKey, err := s.apiKeys.GetByID(ctx, req.APIKeyID)
	if err != nil || apiKey == nil || apiKey.UserID != userID {
		return nil, ErrWorkbenchAPIKeyNotFound
	}
	if apiKey.Status != StatusAPIKeyActive {
		return nil, ErrWorkbenchAPIKeyUnavailable
	}

	apiKeyID := apiKey.ID
	options := nonNilWorkbenchMap(req.Options)
	model := strings.TrimSpace(req.Model)
	var chatHistory []WorkbenchMessage
	if mode == WorkbenchModeChat {
		chatHistory, err = s.repo.ListRecentChatMessages(ctx, userID, conversationID, workbenchHistoryLimit)
		if err != nil {
			return nil, err
		}
	}
	userMessage := WorkbenchMessage{
		ConversationID: conversationID,
		UserID:         userID,
		Mode:           mode,
		Role:           WorkbenchRoleUser,
		Content:        input,
		APIKeyID:       &apiKeyID,
		Endpoint:       endpoint,
		Model:          model,
		RequestOptions: options,
		Status:         WorkbenchMessageStatusSuccess,
	}
	if err := s.repo.CreateMessage(ctx, &userMessage); err != nil {
		return nil, err
	}

	assistantMessage := WorkbenchMessage{
		ConversationID: conversationID,
		UserID:         userID,
		Mode:           mode,
		Role:           WorkbenchRoleAssistant,
		APIKeyID:       &apiKeyID,
		Endpoint:       endpoint,
		Model:          model,
		RequestOptions: options,
		Status:         WorkbenchMessageStatusSuccess,
	}

	var sendErr error
	if mode == WorkbenchModeImage {
		var resp WorkbenchGatewayImageResponse
		resp, sendErr = s.gateway.GenerateImage(ctx, "Bearer "+apiKey.Key, WorkbenchGatewayImageRequest{
			Model:   model,
			Prompt:  input,
			Options: options,
		})
		assistantMessage.ImageOutputs = resp.Images
		assistantMessage.ResponseMetadata = nonNilWorkbenchMap(resp.Metadata)
		assistantMessage.Content = imageAssistantContent(resp.Images)
	} else {
		var resp WorkbenchGatewayChatResponse
		resp, sendErr = s.gateway.SendChat(ctx, "Bearer "+apiKey.Key, WorkbenchGatewayChatRequest{
			Model:    model,
			Messages: append(buildWorkbenchChatMessages(chatHistory), WorkbenchGatewayMessage{Role: WorkbenchRoleUser, Content: input}),
			Options:  options,
		})
		assistantMessage.Content = truncateWorkbenchText(resp.Content, workbenchMessageContentMax)
		assistantMessage.ResponseMetadata = nonNilWorkbenchMap(resp.Metadata)
	}

	if sendErr != nil {
		msg := sanitizeWorkbenchError(sendErr.Error(), apiKey.Key)
		assistantMessage.Status = WorkbenchMessageStatusError
		assistantMessage.Content = ""
		assistantMessage.ErrorMessage = &msg
	}
	if err := s.repo.CreateMessage(ctx, &assistantMessage); err != nil {
		return nil, err
	}

	update := WorkbenchConversationUpdate{
		UserID:             userID,
		ConversationID:     conversationID,
		Mode:               mode,
		APIKeyID:           &apiKeyID,
		Endpoint:           endpoint,
		Model:              model,
		Title:              conversationTitle(*conv, input, mode),
		LastMessagePreview: conversationPreview(input, assistantMessage),
		LastError:          assistantMessage.ErrorMessage,
		MessageCountDelta:  2,
	}
	if err := s.repo.UpdateConversationAfterMessage(ctx, update); err != nil {
		return nil, err
	}

	updated, err := s.repo.GetConversation(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	result := &WorkbenchSendResult{
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
		Conversation:     *updated,
	}
	if sendErr != nil {
		return result, sendErr
	}
	return result, nil
}

func normalizeWorkbenchMode(mode string) string {
	if strings.TrimSpace(mode) == WorkbenchModeImage {
		return WorkbenchModeImage
	}
	return WorkbenchModeChat
}

func normalizeWorkbenchEndpoint(endpoint, mode string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint != "" {
		return endpoint
	}
	if mode == WorkbenchModeImage {
		return WorkbenchEndpointImagesGenerations
	}
	return WorkbenchEndpointChatCompletions
}

func validateWorkbenchModeEndpoint(mode, endpoint string) error {
	switch mode {
	case WorkbenchModeChat:
		if endpoint != WorkbenchEndpointChatCompletions {
			return ErrWorkbenchInvalidEndpoint
		}
	case WorkbenchModeImage:
		if endpoint != WorkbenchEndpointImagesGenerations {
			return ErrWorkbenchInvalidEndpoint
		}
	default:
		return ErrWorkbenchInvalidMode
	}
	return nil
}

func truncateWorkbenchText(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func sanitizeWorkbenchError(message, secret string) string {
	message = strings.TrimSpace(message)
	if secret != "" {
		message = strings.ReplaceAll(message, secret, "[redacted]")
	}
	return truncateWorkbenchText(message, workbenchErrorMessageMax)
}

func nonNilWorkbenchMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

func buildWorkbenchChatMessages(history []WorkbenchMessage) []WorkbenchGatewayMessage {
	out := make([]WorkbenchGatewayMessage, 0, len(history))
	for _, m := range history {
		if m.Mode != WorkbenchModeChat || m.Status != WorkbenchMessageStatusSuccess {
			continue
		}
		if m.Role != WorkbenchRoleUser && m.Role != WorkbenchRoleAssistant {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		out = append(out, WorkbenchGatewayMessage{Role: m.Role, Content: content})
	}
	return out
}

func imageAssistantContent(images []WorkbenchImageOutput) string {
	if len(images) == 0 {
		return "未返回图片"
	}
	return "已生成图片"
}

func conversationTitle(conv WorkbenchConversation, input, mode string) string {
	if conv.MessageCount > 0 && strings.TrimSpace(conv.Title) != "" {
		return conv.Title
	}
	title := truncateWorkbenchText(strings.TrimSpace(input), 40)
	if title == "" {
		if mode == WorkbenchModeImage {
			return "新生图会话"
		}
		return "新对话"
	}
	return title
}

func conversationPreview(input string, assistant WorkbenchMessage) string {
	if assistant.Status == WorkbenchMessageStatusError && assistant.ErrorMessage != nil {
		return truncateWorkbenchText(*assistant.ErrorMessage, workbenchConversationPreviewMax)
	}
	if strings.TrimSpace(assistant.Content) != "" {
		return truncateWorkbenchText(assistant.Content, workbenchConversationPreviewMax)
	}
	return truncateWorkbenchText(input, workbenchConversationPreviewMax)
}
