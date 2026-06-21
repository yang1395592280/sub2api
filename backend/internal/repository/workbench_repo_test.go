package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newWorkbenchEntRepo(t *testing.T) (service.WorkbenchRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return NewWorkbenchRepository(client), client
}

func TestWorkbenchRepository(t *testing.T) {
	t.Run("CreateMessage rejects cross user conversation writes", func(t *testing.T) {
		ctx := context.Background()
		repo, client := newWorkbenchEntRepo(t)

		userA := createWorkbenchUser(t, ctx, client, "a@example.com")
		userB := createWorkbenchUser(t, ctx, client, "b@example.com")
		conv := createWorkbenchConversation(t, ctx, repo, userA.ID, "user-a-chat", service.WorkbenchModeChat)

		err := repo.CreateMessage(ctx, &service.WorkbenchMessage{
			UserID:         userB.ID,
			ConversationID: conv.ID,
			Mode:           service.WorkbenchModeChat,
			Role:           service.WorkbenchRoleUser,
			Content:        "nope",
			Endpoint:       service.WorkbenchEndpointChatCompletions,
			Model:          "gpt-5.5",
			Status:         service.WorkbenchMessageStatusSuccess,
		})
		require.ErrorIs(t, err, service.ErrWorkbenchConversationNotFound)

		messages, listErr := repo.ListMessages(ctx, userA.ID, conv.ID)
		require.NoError(t, listErr)
		require.Empty(t, messages)
	})

	t.Run("CreateMessage rejects soft deleted conversations", func(t *testing.T) {
		ctx := context.Background()
		repo, client := newWorkbenchEntRepo(t)

		user := createWorkbenchUser(t, ctx, client, "soft-delete@example.com")
		conv := createWorkbenchConversation(t, ctx, repo, user.ID, "soft-delete-chat", service.WorkbenchModeChat)

		require.NoError(t, repo.SoftDeleteConversation(ctx, user.ID, conv.ID))

		err := repo.CreateMessage(ctx, &service.WorkbenchMessage{
			UserID:         user.ID,
			ConversationID: conv.ID,
			Mode:           service.WorkbenchModeChat,
			Role:           service.WorkbenchRoleUser,
			Content:        "still nope",
			Endpoint:       service.WorkbenchEndpointChatCompletions,
			Model:          "gpt-5.5",
			Status:         service.WorkbenchMessageStatusSuccess,
		})
		require.ErrorIs(t, err, service.ErrWorkbenchConversationNotFound)
	})

	t.Run("ListRecentChatMessages filters successful chat messages and restores ascending order", func(t *testing.T) {
		ctx := context.Background()
		repo, client := newWorkbenchEntRepo(t)

		user := createWorkbenchUser(t, ctx, client, "recent@example.com")
		conv := createWorkbenchConversation(t, ctx, repo, user.ID, "recent-chat", service.WorkbenchModeChat)

		messageSpecs := []struct {
			content string
			mode    string
			status  string
		}{
			{content: "chat-1", mode: service.WorkbenchModeChat, status: service.WorkbenchMessageStatusSuccess},
			{content: "image-ignored", mode: service.WorkbenchModeImage, status: service.WorkbenchMessageStatusSuccess},
			{content: "chat-error", mode: service.WorkbenchModeChat, status: service.WorkbenchMessageStatusError},
			{content: "chat-2", mode: service.WorkbenchModeChat, status: service.WorkbenchMessageStatusSuccess},
			{content: "chat-3", mode: service.WorkbenchModeChat, status: service.WorkbenchMessageStatusSuccess},
		}

		for i, spec := range messageSpecs {
			msg := &service.WorkbenchMessage{
				UserID:         user.ID,
				ConversationID: conv.ID,
				Mode:           spec.mode,
				Role:           service.WorkbenchRoleUser,
				Content:        spec.content,
				Endpoint:       service.WorkbenchEndpointChatCompletions,
				Model:          "gpt-5.5",
				Status:         spec.status,
			}
			require.NoError(t, repo.CreateMessage(ctx, msg))
			if i < len(messageSpecs)-1 {
				time.Sleep(2 * time.Millisecond)
			}
		}

		messages, err := repo.ListRecentChatMessages(ctx, user.ID, conv.ID, 2)
		require.NoError(t, err)
		require.Len(t, messages, 2)
		require.Equal(t, []string{"chat-2", "chat-3"}, []string{messages[0].Content, messages[1].Content})
	})

	t.Run("UpdateConversationAfterMessage updates fields and rejects deleted conversations", func(t *testing.T) {
		ctx := context.Background()
		repo, client := newWorkbenchEntRepo(t)

		user := createWorkbenchUser(t, ctx, client, "update@example.com")
		conv := createWorkbenchConversation(t, ctx, repo, user.ID, "before", service.WorkbenchModeChat)
		apiKeyID := int64(42)
		lastError := "boom"

		err := repo.UpdateConversationAfterMessage(ctx, service.WorkbenchConversationUpdate{
			UserID:             user.ID,
			ConversationID:     conv.ID,
			Mode:               service.WorkbenchModeImage,
			APIKeyID:           &apiKeyID,
			Endpoint:           service.WorkbenchEndpointImagesGenerations,
			Model:              "gpt-image-1",
			Title:              "after",
			LastMessagePreview: "preview",
			LastError:          &lastError,
			MessageCountDelta:  2,
		})
		require.NoError(t, err)

		updated, err := repo.GetConversation(ctx, user.ID, conv.ID)
		require.NoError(t, err)
		require.Equal(t, "after", updated.Title)
		require.Equal(t, service.WorkbenchModeImage, updated.Mode)
		require.NotNil(t, updated.APIKeyID)
		require.Equal(t, apiKeyID, *updated.APIKeyID)
		require.Equal(t, service.WorkbenchEndpointImagesGenerations, updated.Endpoint)
		require.Equal(t, "gpt-image-1", updated.Model)
		require.Equal(t, "preview", updated.LastMessagePreview)
		require.NotNil(t, updated.LastError)
		require.Equal(t, lastError, *updated.LastError)
		require.Equal(t, 2, updated.MessageCount)

		require.NoError(t, repo.SoftDeleteConversation(ctx, user.ID, conv.ID))

		err = repo.UpdateConversationAfterMessage(ctx, service.WorkbenchConversationUpdate{
			UserID:             user.ID,
			ConversationID:     conv.ID,
			Mode:               service.WorkbenchModeChat,
			Endpoint:           service.WorkbenchEndpointChatCompletions,
			Model:              "gpt-5.5",
			LastMessagePreview: "after delete",
			MessageCountDelta:  1,
		})
		require.ErrorIs(t, err, service.ErrWorkbenchConversationNotFound)
	})

	t.Run("ListConversations applies mode filter and reports total before pagination", func(t *testing.T) {
		ctx := context.Background()
		repo, client := newWorkbenchEntRepo(t)

		user := createWorkbenchUser(t, ctx, client, "list@example.com")
		otherUser := createWorkbenchUser(t, ctx, client, "other@example.com")
		chatA := createWorkbenchConversation(t, ctx, repo, user.ID, "chat-a", service.WorkbenchModeChat)
		_ = createWorkbenchConversation(t, ctx, repo, user.ID, "image-a", service.WorkbenchModeImage)
		chatB := createWorkbenchConversation(t, ctx, repo, user.ID, "chat-b", service.WorkbenchModeChat)
		_ = createWorkbenchConversation(t, ctx, repo, otherUser.ID, "other-chat", service.WorkbenchModeChat)

		base := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
		_, err := client.WorkbenchConversation.UpdateOneID(chatA.ID).
			SetUpdatedAt(base.Add(1 * time.Minute)).
			Save(ctx)
		require.NoError(t, err)
		_, err = client.WorkbenchConversation.UpdateOneID(chatB.ID).
			SetUpdatedAt(base.Add(2 * time.Minute)).
			Save(ctx)
		require.NoError(t, err)

		conversations, page, err := repo.ListConversations(ctx, user.ID, pagination.PaginationParams{
			Page:     1,
			PageSize: 1,
		}, service.WorkbenchConversationFilters{Mode: service.WorkbenchModeChat})
		require.NoError(t, err)
		require.Len(t, conversations, 1)
		require.NotNil(t, page)
		require.Equal(t, int64(2), page.Total)
		require.Equal(t, chatB.ID, conversations[0].ID)
	})
}

func createWorkbenchUser(t *testing.T, ctx context.Context, client *dbent.Client, email string) *dbent.User {
	t.Helper()

	return client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SaveX(ctx)
}

func createWorkbenchConversation(t *testing.T, ctx context.Context, repo service.WorkbenchRepository, userID int64, title, mode string) *service.WorkbenchConversation {
	t.Helper()

	conv := &service.WorkbenchConversation{
		UserID:             userID,
		Title:              title,
		Mode:               mode,
		Endpoint:           service.WorkbenchEndpointChatCompletions,
		Model:              "gpt-5.5",
		LastMessagePreview: title,
	}
	require.NoError(t, repo.CreateConversation(ctx, conv))
	require.NotZero(t, conv.ID)
	return conv
}
