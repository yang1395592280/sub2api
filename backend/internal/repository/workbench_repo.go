package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/workbenchconversation"
	"github.com/Wei-Shaw/sub2api/ent/workbenchmessage"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"entgo.io/ent/dialect/sql"
)

type workbenchRepository struct {
	client *dbent.Client
}

func NewWorkbenchRepository(client *dbent.Client) service.WorkbenchRepository {
	return &workbenchRepository{client: client}
}

func (r *workbenchRepository) CreateConversation(ctx context.Context, c *service.WorkbenchConversation) error {
	client := clientFromContext(ctx, r.client)
	builder := client.WorkbenchConversation.Create().
		SetUserID(c.UserID).
		SetTitle(c.Title).
		SetMode(c.Mode).
		SetEndpoint(c.Endpoint).
		SetModel(c.Model).
		SetLastMessagePreview(c.LastMessagePreview).
		SetMessageCount(c.MessageCount)
	if c.APIKeyID != nil {
		builder.SetAPIKeyID(*c.APIKeyID)
	}
	if c.LastError != nil {
		builder.SetLastError(*c.LastError)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	applyWorkbenchConversation(c, created)
	return nil
}

func (r *workbenchRepository) ListConversations(ctx context.Context, userID int64, params pagination.PaginationParams, filters service.WorkbenchConversationFilters) ([]service.WorkbenchConversation, *pagination.PaginationResult, error) {
	client := clientFromContext(ctx, r.client)
	q := client.WorkbenchConversation.Query().Where(workbenchconversation.UserIDEQ(userID))
	if filters.Mode != "" {
		q = q.Where(workbenchconversation.ModeEQ(filters.Mode))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(workbenchconversation.ByUpdatedAt(sql.OrderDesc())).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.WorkbenchConversation, 0, len(rows))
	for _, row := range rows {
		out = append(out, workbenchConversationFromEnt(row))
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

func (r *workbenchRepository) GetConversation(ctx context.Context, userID, conversationID int64) (*service.WorkbenchConversation, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.WorkbenchConversation.Query().
		Where(workbenchconversation.IDEQ(conversationID), workbenchconversation.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrWorkbenchConversationNotFound, nil)
	}
	out := workbenchConversationFromEnt(row)
	return &out, nil
}

func (r *workbenchRepository) SoftDeleteConversation(ctx context.Context, userID, conversationID int64) error {
	client := clientFromContext(ctx, r.client)
	deletedAt := time.Now().UTC()
	updated, err := client.WorkbenchConversation.Update().
		Where(workbenchconversation.IDEQ(conversationID), workbenchconversation.UserIDEQ(userID)).
		SetDeletedAt(deletedAt).
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return service.ErrWorkbenchConversationNotFound
	}
	_, err = client.WorkbenchMessage.Update().
		Where(workbenchmessage.ConversationIDEQ(conversationID), workbenchmessage.UserIDEQ(userID)).
		SetDeletedAt(deletedAt).
		Save(ctx)
	return err
}

func (r *workbenchRepository) CreateMessage(ctx context.Context, m *service.WorkbenchMessage) error {
	client := clientFromContext(ctx, r.client)
	builder := client.WorkbenchMessage.Create().
		SetConversationID(m.ConversationID).
		SetUserID(m.UserID).
		SetMode(m.Mode).
		SetRole(m.Role).
		SetContent(m.Content).
		SetEndpoint(m.Endpoint).
		SetModel(m.Model).
		SetRequestOptions(nonNilMap(m.RequestOptions)).
		SetResponseMetadata(nonNilMap(m.ResponseMetadata)).
		SetImageOutputs(nonNilImageOutputs(m.ImageOutputs)).
		SetStatus(m.Status)
	if m.APIKeyID != nil {
		builder.SetAPIKeyID(*m.APIKeyID)
	}
	if m.ErrorMessage != nil {
		builder.SetErrorMessage(*m.ErrorMessage)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	applyWorkbenchMessage(m, created)
	return nil
}

func (r *workbenchRepository) ListMessages(ctx context.Context, userID, conversationID int64) ([]service.WorkbenchMessage, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.WorkbenchMessage.Query().
		Where(workbenchmessage.UserIDEQ(userID), workbenchmessage.ConversationIDEQ(conversationID)).
		Order(workbenchmessage.ByCreatedAt(sql.OrderAsc())).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return workbenchMessagesFromEnt(rows), nil
}

func (r *workbenchRepository) ListRecentChatMessages(ctx context.Context, userID, conversationID int64, limit int) ([]service.WorkbenchMessage, error) {
	client := clientFromContext(ctx, r.client)
	if limit <= 0 {
		limit = 20
	}
	rows, err := client.WorkbenchMessage.Query().
		Where(
			workbenchmessage.UserIDEQ(userID),
			workbenchmessage.ConversationIDEQ(conversationID),
			workbenchmessage.ModeEQ(service.WorkbenchModeChat),
			workbenchmessage.StatusEQ(service.WorkbenchMessageStatusSuccess),
		).
		Order(workbenchmessage.ByCreatedAt(sql.OrderDesc())).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := workbenchMessagesFromEnt(rows)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (r *workbenchRepository) UpdateConversationAfterMessage(ctx context.Context, update service.WorkbenchConversationUpdate) error {
	client := clientFromContext(ctx, r.client)
	builder := client.WorkbenchConversation.Update().
		Where(workbenchconversation.IDEQ(update.ConversationID), workbenchconversation.UserIDEQ(update.UserID)).
		SetMode(update.Mode).
		SetEndpoint(update.Endpoint).
		SetModel(update.Model).
		SetLastMessagePreview(update.LastMessagePreview).
		AddMessageCount(update.MessageCountDelta)
	if update.Title != "" {
		builder.SetTitle(update.Title)
	}
	if update.APIKeyID != nil {
		builder.SetAPIKeyID(*update.APIKeyID)
	} else {
		builder.ClearAPIKeyID()
	}
	if update.LastError != nil {
		builder.SetLastError(*update.LastError)
	} else {
		builder.ClearLastError()
	}
	n, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrWorkbenchConversationNotFound
	}
	return nil
}

func workbenchConversationFromEnt(row *dbent.WorkbenchConversation) service.WorkbenchConversation {
	out := service.WorkbenchConversation{
		ID:                 row.ID,
		UserID:             row.UserID,
		Title:              row.Title,
		Mode:               row.Mode,
		Endpoint:           row.Endpoint,
		Model:              row.Model,
		LastMessagePreview: row.LastMessagePreview,
		MessageCount:       row.MessageCount,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
	if row.APIKeyID != nil {
		out.APIKeyID = row.APIKeyID
	}
	if row.LastError != nil {
		out.LastError = row.LastError
	}
	return out
}

func applyWorkbenchConversation(dst *service.WorkbenchConversation, row *dbent.WorkbenchConversation) {
	*dst = workbenchConversationFromEnt(row)
}

func workbenchMessageFromEnt(row *dbent.WorkbenchMessage) service.WorkbenchMessage {
	out := service.WorkbenchMessage{
		ID:               row.ID,
		ConversationID:   row.ConversationID,
		UserID:           row.UserID,
		Mode:             row.Mode,
		Role:             row.Role,
		Content:          row.Content,
		Endpoint:         row.Endpoint,
		Model:            row.Model,
		RequestOptions:   nonNilMap(row.RequestOptions),
		ResponseMetadata: nonNilMap(row.ResponseMetadata),
		ImageOutputs:     nonNilImageOutputs(row.ImageOutputs),
		Status:           row.Status,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
	if row.APIKeyID != nil {
		out.APIKeyID = row.APIKeyID
	}
	if row.ErrorMessage != nil {
		out.ErrorMessage = row.ErrorMessage
	}
	return out
}

func workbenchMessagesFromEnt(rows []*dbent.WorkbenchMessage) []service.WorkbenchMessage {
	out := make([]service.WorkbenchMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, workbenchMessageFromEnt(row))
	}
	return out
}

func applyWorkbenchMessage(dst *service.WorkbenchMessage, row *dbent.WorkbenchMessage) {
	*dst = workbenchMessageFromEnt(row)
}

func nonNilMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

func nonNilImageOutputs(in []service.WorkbenchImageOutput) []service.WorkbenchImageOutput {
	if in == nil {
		return []service.WorkbenchImageOutput{}
	}
	return in
}
