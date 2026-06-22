package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	workbenchHistoryLimit           = 20
	workbenchConversationTitleMax   = 160
	workbenchConversationPreviewMax = 120
	workbenchMessageContentMax      = 12000
	workbenchErrorMessageMax        = 500
	defaultWorkbenchImageModel      = "gpt-image-2"
	workbenchImagePendingContent    = "生图任务已提交，正在生成图片。"
	workbenchAsyncImageTimeout      = 5 * time.Minute
)

type WorkbenchService struct {
	repo          WorkbenchRepository
	apiKeys       WorkbenchAPIKeyLookup
	gateway       WorkbenchGatewayClient
	modelProvider WorkbenchModelProvider
	asyncRunner   func(func())
}

func NewWorkbenchService(repo WorkbenchRepository, apiKeys WorkbenchAPIKeyLookup, gateway WorkbenchGatewayClient) *WorkbenchService {
	return NewWorkbenchServiceWithModels(repo, apiKeys, gateway, nil)
}

func NewWorkbenchServiceWithModels(repo WorkbenchRepository, apiKeys WorkbenchAPIKeyLookup, gateway WorkbenchGatewayClient, modelProvider WorkbenchModelProvider) *WorkbenchService {
	return &WorkbenchService{
		repo:          repo,
		apiKeys:       apiKeys,
		gateway:       gateway,
		modelProvider: modelProvider,
		asyncRunner:   func(fn func()) { go fn() },
	}
}

func (s *WorkbenchService) ListModels(ctx context.Context, userID, apiKeyID int64) ([]WorkbenchModel, error) {
	apiKey, err := s.validateWorkbenchAPIKey(ctx, userID, apiKeyID)
	if err != nil {
		return nil, err
	}

	platform := ""
	if apiKey.Group != nil {
		platform = apiKey.Group.Platform
	}
	models := []string(nil)
	if s.modelProvider != nil {
		models = s.modelProvider.GetAvailableModels(ctx, apiKey.GroupID, platform)
	}
	if apiKey.Group != nil && apiKey.Group.CustomModelsListEnabled() {
		models = filterWorkbenchModelsByCustomList(models, defaultWorkbenchModelIDsForPlatform(platform), apiKey.Group.ModelsListConfig.Models)
	} else if len(models) == 0 {
		models = defaultWorkbenchModelIDsForPlatform(platform)
	}

	out := make([]WorkbenchModel, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, WorkbenchModel{Name: model})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
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

	model := strings.TrimSpace(req.Model)
	var apiKeyID *int64
	if req.APIKeyID != nil {
		apiKey, err := s.validateWorkbenchAPIKey(ctx, userID, *req.APIKeyID)
		if err != nil {
			return nil, err
		}
		apiKeyID = &apiKey.ID
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
		APIKeyID: apiKeyID,
		Endpoint: endpoint,
		Model:    model,
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

func (s *WorkbenchService) AdminListConversations(ctx context.Context, params pagination.PaginationParams, filters AdminWorkbenchConversationFilters) ([]AdminWorkbenchConversation, *pagination.PaginationResult, error) {
	if filters.Mode != "" && filters.Mode != WorkbenchModeChat && filters.Mode != WorkbenchModeImage {
		return nil, nil, ErrWorkbenchInvalidMode
	}
	if filters.Status != "" && filters.Status != WorkbenchMessageStatusPending && filters.Status != WorkbenchMessageStatusSuccess && filters.Status != WorkbenchMessageStatusError {
		return nil, nil, ErrWorkbenchInvalidStatus
	}
	return s.repo.AdminListConversations(ctx, params, filters)
}

func (s *WorkbenchService) AdminGetConversation(ctx context.Context, conversationID int64) (*AdminWorkbenchConversation, []WorkbenchMessage, error) {
	return s.repo.AdminGetConversation(ctx, conversationID)
}

func (s *WorkbenchService) AdminGetStats(ctx context.Context, retentionDays int) (*AdminWorkbenchStats, error) {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	return s.repo.AdminGetStats(ctx, retentionDays)
}

func (s *WorkbenchService) AdminHardDeleteConversations(ctx context.Context, conversationIDs []int64) (int64, error) {
	ids := make([]int64, 0, len(conversationIDs))
	seen := make(map[int64]struct{}, len(conversationIDs))
	for _, id := range conversationIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return s.repo.AdminHardDeleteConversations(ctx, ids)
}

func (s *WorkbenchService) AdminHardDeleteExpiredConversations(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	return s.repo.AdminHardDeleteExpiredConversations(ctx, cutoff)
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

	model := strings.TrimSpace(req.Model)
	if mode == WorkbenchModeImage {
		model = normalizeWorkbenchImageModel(model)
	}
	if model == "" {
		return nil, ErrWorkbenchEmptyModel
	}

	conv, err := s.repo.GetConversation(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}

	apiKey, err := s.validateWorkbenchAPIKey(ctx, userID, req.APIKeyID)
	if err != nil {
		return nil, err
	}

	apiKeyID := apiKey.ID
	options := nonNilWorkbenchMap(req.Options)
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

	if mode == WorkbenchModeImage {
		assistantMessage.Status = WorkbenchMessageStatusPending
		assistantMessage.Content = workbenchImagePendingContent
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
			LastError:          nil,
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
		s.runAsync(func() {
			s.completeImageMessage(apiKey.Key, input, model, options, req.PublicHost, req.PublicScheme, *conv, assistantMessage)
		})
		return result, nil
	}

	var sendErr error
	var resp WorkbenchGatewayChatResponse
	resp, sendErr = s.gateway.SendChat(ctx, "Bearer "+apiKey.Key, WorkbenchGatewayChatRequest{
		Model:    model,
		Messages: append(buildWorkbenchChatMessages(chatHistory), WorkbenchGatewayMessage{Role: WorkbenchRoleUser, Content: input}),
		Options:  options,
	})
	assistantMessage.Content = truncateWorkbenchText(resp.Content, workbenchMessageContentMax)
	assistantMessage.ResponseMetadata = nonNilWorkbenchMap(resp.Metadata)

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

func (s *WorkbenchService) runAsync(fn func()) {
	if s != nil && s.asyncRunner != nil {
		s.asyncRunner(fn)
		return
	}
	go fn()
}

func (s *WorkbenchService) completeImageMessage(apiKeySecret, input, model string, options map[string]any, publicHost, publicScheme string, conv WorkbenchConversation, assistantMessage WorkbenchMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), workbenchAsyncImageTimeout)
	defer cancel()

	resp, sendErr := s.gateway.GenerateImage(ctx, "Bearer "+apiKeySecret, WorkbenchGatewayImageRequest{
		Model:        model,
		Prompt:       input,
		Options:      options,
		PublicHost:   strings.TrimSpace(publicHost),
		PublicScheme: strings.TrimSpace(publicScheme),
	})

	completed := WorkbenchMessageUpdate{
		UserID:           assistantMessage.UserID,
		ConversationID:   assistantMessage.ConversationID,
		MessageID:        assistantMessage.ID,
		ResponseMetadata: nonNilWorkbenchMap(resp.Metadata),
		ImageOutputs:     resp.Images,
		Status:           WorkbenchMessageStatusSuccess,
		Content:          imageAssistantContent(resp.Images),
	}
	if sendErr != nil {
		msg := sanitizeWorkbenchError(sendErr.Error(), apiKeySecret)
		completed.Status = WorkbenchMessageStatusError
		completed.Content = ""
		completed.ImageOutputs = nil
		completed.ErrorMessage = &msg
	}
	if err := s.repo.UpdateMessageAfterGateway(ctx, completed); err != nil {
		return
	}

	_ = s.repo.UpdateConversationAfterMessage(ctx, WorkbenchConversationUpdate{
		UserID:             assistantMessage.UserID,
		ConversationID:     assistantMessage.ConversationID,
		Mode:               assistantMessage.Mode,
		APIKeyID:           assistantMessage.APIKeyID,
		Endpoint:           assistantMessage.Endpoint,
		Model:              assistantMessage.Model,
		Title:              conversationTitle(conv, input, assistantMessage.Mode),
		LastMessagePreview: conversationPreview(input, WorkbenchMessage{Content: completed.Content, Status: completed.Status, ErrorMessage: completed.ErrorMessage}),
		LastError:          completed.ErrorMessage,
		MessageCountDelta:  0,
	})
}

func (s *WorkbenchService) validateWorkbenchAPIKey(ctx context.Context, userID, apiKeyID int64) (*APIKey, error) {
	apiKey, err := s.apiKeys.GetByID(ctx, apiKeyID)
	if err != nil || apiKey == nil || apiKey.UserID != userID {
		return nil, ErrWorkbenchAPIKeyNotFound
	}
	if apiKey.Status != StatusAPIKeyActive {
		return nil, ErrWorkbenchAPIKeyUnavailable
	}
	return apiKey, nil
}

func defaultWorkbenchModelIDsForPlatform(platform string) []string {
	switch platform {
	case PlatformOpenAI:
		return openai.DefaultModelIDs()
	case PlatformGemini:
		ids := make([]string, 0, len(geminicli.DefaultModels))
		for _, model := range geminicli.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	case PlatformAntigravity:
		models := antigravity.DefaultModels()
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}
		return ids
	default:
		return claude.DefaultModelIDs()
	}
}

func filterWorkbenchModelsByCustomList(availableModels, fallbackModels, selectedModels []string) []string {
	if len(selectedModels) == 0 {
		return availableModels
	}
	source := availableModels
	if len(source) == 0 {
		source = fallbackModels
	}
	if len(source) == 0 {
		return nil
	}

	allowed := make([]string, 0, len(source))
	for _, model := range source {
		model = strings.TrimSpace(model)
		if model != "" {
			allowed = append(allowed, model)
		}
	}

	seen := make(map[string]struct{}, len(selectedModels))
	filtered := make([]string, 0, len(selectedModels))
	for _, model := range selectedModels {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if !workbenchModelsListAllowsModel(allowed, model) {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		filtered = append(filtered, model)
	}
	return filtered
}

func workbenchModelsListAllowsModel(availablePatterns []string, model string) bool {
	for _, pattern := range availablePatterns {
		if pattern == model {
			return true
		}
		if strings.HasSuffix(pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}
	return false
}

func normalizeWorkbenchMode(mode string) string {
	mode = strings.TrimSpace(mode)
	switch mode {
	case "":
		return WorkbenchModeChat
	case WorkbenchModeChat:
		return WorkbenchModeChat
	case WorkbenchModeImage:
		return WorkbenchModeImage
	default:
		return mode
	}
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

func normalizeWorkbenchImageModel(model string) string {
	model = strings.TrimSpace(model)
	if strings.HasPrefix(strings.ToLower(model), "gpt-image-") {
		return model
	}
	return defaultWorkbenchImageModel
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
	const fallback = "workbench gateway request failed"

	message = strings.TrimSpace(message)
	if message == "" {
		return fallback
	}
	if strings.HasPrefix(message, "gateway returned ") {
		fields := strings.Fields(message)
		if len(fields) >= 3 {
			return truncateWorkbenchText(strings.TrimSuffix(strings.Join(fields[:3], " "), ":"), workbenchErrorMessageMax)
		}
		return fallback
	}
	if secret != "" {
		message = strings.ReplaceAll(message, secret, "[redacted]")
	}
	return fallback
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
