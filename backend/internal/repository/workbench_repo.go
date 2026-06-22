package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/ent/user"
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
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		deletedAt := time.Now().UTC()
		updated, err := txClient.WorkbenchConversation.Update().
			Where(
				workbenchconversation.IDEQ(conversationID),
				workbenchconversation.UserIDEQ(userID),
				workbenchconversation.DeletedAtIsNil(),
			).
			SetDeletedAt(deletedAt).
			Save(txCtx)
		if err != nil {
			return err
		}
		if updated == 0 {
			return service.ErrWorkbenchConversationNotFound
		}
		_, err = txClient.WorkbenchMessage.Update().
			Where(
				workbenchmessage.ConversationIDEQ(conversationID),
				workbenchmessage.UserIDEQ(userID),
				workbenchmessage.DeletedAtIsNil(),
			).
			SetDeletedAt(deletedAt).
			Save(txCtx)
		return err
	})
}

func (r *workbenchRepository) CreateMessage(ctx context.Context, m *service.WorkbenchMessage) error {
	var created *dbent.WorkbenchMessage
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		_, err := txClient.WorkbenchConversation.Query().
			Where(
				workbenchconversation.IDEQ(m.ConversationID),
				workbenchconversation.UserIDEQ(m.UserID),
				workbenchconversation.DeletedAtIsNil(),
			).
			Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrWorkbenchConversationNotFound, nil)
		}

		builder := txClient.WorkbenchMessage.Create().
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
		created, err = builder.Save(txCtx)
		return err
	})
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

func (r *workbenchRepository) AdminListConversations(ctx context.Context, params pagination.PaginationParams, filters service.AdminWorkbenchConversationFilters) ([]service.AdminWorkbenchConversation, *pagination.PaginationResult, error) {
	client := clientFromContext(ctx, r.client)
	q := client.WorkbenchConversation.Query().WithUser()
	q = applyAdminWorkbenchConversationFilters(q, filters)

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
	out := r.adminWorkbenchConversationsFromEnt(ctx, rows)
	return out, paginationResultFromTotal(int64(total), params), nil
}

func (r *workbenchRepository) AdminGetConversation(ctx context.Context, conversationID int64) (*service.AdminWorkbenchConversation, []service.WorkbenchMessage, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.WorkbenchConversation.Query().
		Where(workbenchconversation.IDEQ(conversationID)).
		WithUser().
		Only(ctx)
	if err != nil {
		return nil, nil, translatePersistenceError(err, service.ErrWorkbenchConversationNotFound, nil)
	}
	out := r.adminWorkbenchConversationsFromEnt(ctx, []*dbent.WorkbenchConversation{row})
	if len(out) == 0 {
		return nil, nil, service.ErrWorkbenchConversationNotFound
	}
	messages, err := client.WorkbenchMessage.Query().
		Where(workbenchmessage.ConversationIDEQ(conversationID)).
		Order(workbenchmessage.ByCreatedAt(sql.OrderAsc())).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	return &out[0], workbenchMessagesFromEnt(messages), nil
}

func (r *workbenchRepository) AdminGetStats(ctx context.Context, retentionDays int) (*service.AdminWorkbenchStats, error) {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	client := clientFromContext(ctx, r.client)
	totalConversations, err := client.WorkbenchConversation.Query().Count(ctx)
	if err != nil {
		return nil, err
	}
	totalMessages, err := client.WorkbenchMessage.Query().Count(ctx)
	if err != nil {
		return nil, err
	}
	imageMessages, err := client.WorkbenchMessage.Query().
		Where(workbenchmessage.ModeEQ(service.WorkbenchModeImage)).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	expired, err := client.WorkbenchConversation.Query().
		Where(workbenchconversation.UpdatedAtLT(cutoff)).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	imageBytes, err := r.sumWorkbenchImageBytes(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &service.AdminWorkbenchStats{
		TotalConversations:   int64(totalConversations),
		TotalMessages:        int64(totalMessages),
		ImageMessages:        int64(imageMessages),
		ExpiredConversations: int64(expired),
		ImageBytes:           imageBytes,
		RetentionDays:        retentionDays,
	}, nil
}

func (r *workbenchRepository) AdminHardDeleteConversations(ctx context.Context, conversationIDs []int64) (int64, error) {
	if len(conversationIDs) == 0 {
		return 0, nil
	}
	client := clientFromContext(ctx, r.client)
	deleted, err := client.WorkbenchConversation.Delete().
		Where(workbenchconversation.IDIn(conversationIDs...)).
		Exec(mixins.SkipSoftDelete(ctx))
	return int64(deleted), err
}

func (r *workbenchRepository) AdminHardDeleteExpiredConversations(ctx context.Context, cutoff time.Time) (int64, error) {
	client := clientFromContext(ctx, r.client)
	deleted, err := client.WorkbenchConversation.Delete().
		Where(workbenchconversation.UpdatedAtLT(cutoff)).
		Exec(mixins.SkipSoftDelete(ctx))
	return int64(deleted), err
}

func (r *workbenchRepository) UpdateMessageAfterGateway(ctx context.Context, update service.WorkbenchMessageUpdate) error {
	client := clientFromContext(ctx, r.client)
	builder := client.WorkbenchMessage.Update().
		Where(
			workbenchmessage.IDEQ(update.MessageID),
			workbenchmessage.UserIDEQ(update.UserID),
			workbenchmessage.ConversationIDEQ(update.ConversationID),
			workbenchmessage.DeletedAtIsNil(),
		).
		SetContent(update.Content).
		SetResponseMetadata(nonNilMap(update.ResponseMetadata)).
		SetImageOutputs(nonNilImageOutputs(update.ImageOutputs)).
		SetStatus(update.Status)
	if update.ErrorMessage != nil {
		builder.SetErrorMessage(*update.ErrorMessage)
	} else {
		builder.ClearErrorMessage()
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

func (r *workbenchRepository) UpdateConversationAfterMessage(ctx context.Context, update service.WorkbenchConversationUpdate) error {
	client := clientFromContext(ctx, r.client)
	builder := client.WorkbenchConversation.Update().
		Where(
			workbenchconversation.IDEQ(update.ConversationID),
			workbenchconversation.UserIDEQ(update.UserID),
			workbenchconversation.DeletedAtIsNil(),
		).
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

func (r *workbenchRepository) withTx(ctx context.Context, fn func(txCtx context.Context, txClient *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin workbench transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workbench transaction: %w", err)
	}
	return nil
}

func applyAdminWorkbenchConversationFilters(q *dbent.WorkbenchConversationQuery, filters service.AdminWorkbenchConversationFilters) *dbent.WorkbenchConversationQuery {
	if filters.Mode != "" {
		q = q.Where(workbenchconversation.ModeEQ(filters.Mode))
	}
	if filters.UserID > 0 {
		q = q.Where(workbenchconversation.UserIDEQ(filters.UserID))
	}
	if filters.Search != "" {
		search := strings.TrimSpace(filters.Search)
		q = q.Where(workbenchconversation.Or(
			workbenchconversation.TitleContainsFold(search),
			workbenchconversation.LastMessagePreviewContainsFold(search),
			workbenchconversation.HasUserWith(user.EmailContainsFold(search)),
		))
	}
	if filters.Status != "" {
		q = q.Where(workbenchconversation.HasMessagesWith(workbenchmessage.StatusEQ(filters.Status)))
	}
	if filters.HasImages != nil && *filters.HasImages {
		q = q.Where(workbenchconversation.HasMessagesWith(workbenchmessage.ModeEQ(service.WorkbenchModeImage)))
	}
	if filters.OlderThanDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -filters.OlderThanDays)
		q = q.Where(workbenchconversation.UpdatedAtLT(cutoff))
	}
	return q
}

func (r *workbenchRepository) adminWorkbenchConversationsFromEnt(ctx context.Context, rows []*dbent.WorkbenchConversation) []service.AdminWorkbenchConversation {
	out := make([]service.AdminWorkbenchConversation, 0, len(rows))
	if len(rows) == 0 {
		return out
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	imageCounts, imageBytes := r.workbenchImageStatsByConversation(ctx, ids)
	for _, row := range rows {
		item := service.AdminWorkbenchConversation{WorkbenchConversation: workbenchConversationFromEnt(row)}
		if row.Edges.User != nil {
			item.UserEmail = row.Edges.User.Email
			item.Username = row.Edges.User.Username
		}
		item.ImageCount = imageCounts[row.ID]
		item.ImageBytes = imageBytes[row.ID]
		out = append(out, item)
	}
	return out
}

func (r *workbenchRepository) workbenchImageStatsByConversation(ctx context.Context, conversationIDs []int64) (map[int64]int, map[int64]int64) {
	counts := make(map[int64]int, len(conversationIDs))
	bytesByID := make(map[int64]int64, len(conversationIDs))
	client := clientFromContext(ctx, r.client)
	rows, err := client.WorkbenchMessage.Query().
		Where(workbenchmessage.ConversationIDIn(conversationIDs...), workbenchmessage.ModeEQ(service.WorkbenchModeImage)).
		All(ctx)
	if err != nil {
		return counts, bytesByID
	}
	for _, row := range rows {
		for _, image := range row.ImageOutputs {
			if image.URL != "" || image.B64JSON != "" {
				counts[row.ConversationID]++
			}
			if image.B64JSON != "" {
				bytesByID[row.ConversationID] += int64(len(image.B64JSON))
			}
		}
	}
	return counts, bytesByID
}

func (r *workbenchRepository) sumWorkbenchImageBytes(ctx context.Context, conversationIDs []int64) (int64, error) {
	client := clientFromContext(ctx, r.client)
	q := client.WorkbenchMessage.Query().Where(workbenchmessage.ModeEQ(service.WorkbenchModeImage))
	if len(conversationIDs) > 0 {
		q = q.Where(workbenchmessage.ConversationIDIn(conversationIDs...))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, row := range rows {
		for _, image := range row.ImageOutputs {
			total += int64(len(image.B64JSON))
		}
	}
	return total, nil
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
