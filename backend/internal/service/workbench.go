package service

import (
	"context"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	WorkbenchModeChat  = "chat"
	WorkbenchModeImage = "image"

	WorkbenchEndpointChatCompletions   = "chat_completions"
	WorkbenchEndpointImagesGenerations = "images_generations"

	WorkbenchRoleUser      = "user"
	WorkbenchRoleAssistant = "assistant"
	WorkbenchRoleSystem    = "system"

	WorkbenchMessageStatusPending = "pending"
	WorkbenchMessageStatusSuccess = "success"
	WorkbenchMessageStatusError   = "error"
)

var (
	ErrWorkbenchConversationNotFound = infraerrors.NotFound("WORKBENCH_CONVERSATION_NOT_FOUND", "workbench conversation not found")
	ErrWorkbenchAPIKeyNotFound       = infraerrors.NotFound("WORKBENCH_API_KEY_NOT_FOUND", "api key not found")
	ErrWorkbenchAPIKeyUnavailable    = infraerrors.Forbidden("WORKBENCH_API_KEY_UNAVAILABLE", "api key is not available")
	ErrWorkbenchInvalidMode          = infraerrors.New(http.StatusBadRequest, "WORKBENCH_INVALID_MODE", "invalid workbench mode")
	ErrWorkbenchInvalidEndpoint      = infraerrors.New(http.StatusBadRequest, "WORKBENCH_INVALID_ENDPOINT", "invalid workbench endpoint")
	ErrWorkbenchEmptyInput           = infraerrors.New(http.StatusBadRequest, "WORKBENCH_EMPTY_INPUT", "input is required")
)

type WorkbenchConversation struct {
	ID                 int64     `json:"id"`
	UserID             int64     `json:"user_id"`
	Title              string    `json:"title"`
	Mode               string    `json:"mode"`
	APIKeyID           *int64    `json:"api_key_id,omitempty"`
	Endpoint           string    `json:"endpoint"`
	Model              string    `json:"model"`
	LastMessagePreview string    `json:"last_message_preview"`
	LastError          *string   `json:"last_error,omitempty"`
	MessageCount       int       `json:"message_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type WorkbenchMessage struct {
	ID               int64                  `json:"id"`
	ConversationID   int64                  `json:"conversation_id"`
	UserID           int64                  `json:"user_id"`
	Mode             string                 `json:"mode"`
	Role             string                 `json:"role"`
	Content          string                 `json:"content"`
	APIKeyID         *int64                 `json:"api_key_id,omitempty"`
	Endpoint         string                 `json:"endpoint"`
	Model            string                 `json:"model"`
	RequestOptions   map[string]any         `json:"request_options"`
	ResponseMetadata map[string]any         `json:"response_metadata"`
	ImageOutputs     []WorkbenchImageOutput `json:"image_outputs"`
	Status           string                 `json:"status"`
	ErrorMessage     *string                `json:"error_message,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type WorkbenchImageOutput = domain.WorkbenchImageOutput

type WorkbenchConversationFilters struct {
	Mode string
}

type WorkbenchConversationUpdate struct {
	UserID             int64
	ConversationID     int64
	Mode               string
	APIKeyID           *int64
	Endpoint           string
	Model              string
	Title              string
	LastMessagePreview string
	LastError          *string
	MessageCountDelta  int
}

type WorkbenchAPIKeyLookup interface {
	GetByID(ctx context.Context, id int64) (*APIKey, error)
}

type CreateWorkbenchConversationRequest struct {
	Mode     string
	Title    string
	APIKeyID *int64
	Endpoint string
	Model    string
}

type WorkbenchSendRequest struct {
	Mode     string
	APIKeyID int64
	Endpoint string
	Model    string
	Input    string
	Options  map[string]any
}

type WorkbenchSendResult struct {
	UserMessage      WorkbenchMessage      `json:"user_message"`
	AssistantMessage WorkbenchMessage      `json:"assistant_message"`
	Conversation     WorkbenchConversation `json:"conversation"`
}

type WorkbenchGatewayMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type WorkbenchGatewayChatRequest struct {
	Model    string                    `json:"model"`
	Messages []WorkbenchGatewayMessage `json:"messages"`
	Options  map[string]any            `json:"options"`
}

type WorkbenchGatewayChatResponse struct {
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata"`
}

type WorkbenchGatewayImageRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Options map[string]any `json:"options"`
}

type WorkbenchGatewayImageResponse struct {
	Images   []WorkbenchImageOutput `json:"images"`
	Metadata map[string]any         `json:"metadata"`
}

type WorkbenchGatewayClient interface {
	SendChat(ctx context.Context, authorization string, req WorkbenchGatewayChatRequest) (WorkbenchGatewayChatResponse, error)
	GenerateImage(ctx context.Context, authorization string, req WorkbenchGatewayImageRequest) (WorkbenchGatewayImageResponse, error)
}

type WorkbenchRepository interface {
	CreateConversation(ctx context.Context, c *WorkbenchConversation) error
	ListConversations(ctx context.Context, userID int64, params pagination.PaginationParams, filters WorkbenchConversationFilters) ([]WorkbenchConversation, *pagination.PaginationResult, error)
	GetConversation(ctx context.Context, userID, conversationID int64) (*WorkbenchConversation, error)
	SoftDeleteConversation(ctx context.Context, userID, conversationID int64) error
	CreateMessage(ctx context.Context, m *WorkbenchMessage) error
	ListMessages(ctx context.Context, userID, conversationID int64) ([]WorkbenchMessage, error)
	ListRecentChatMessages(ctx context.Context, userID, conversationID int64, limit int) ([]WorkbenchMessage, error)
	UpdateConversationAfterMessage(ctx context.Context, update WorkbenchConversationUpdate) error
}
