package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

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

func TestWorkbenchRepositoryUserIsolationAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	repo, client := newWorkbenchEntRepo(t)

	userA := client.User.Create().
		SetEmail("a@example.com").
		SetPasswordHash("hash").
		SaveX(ctx)
	userB := client.User.Create().
		SetEmail("b@example.com").
		SetPasswordHash("hash").
		SaveX(ctx)

	conv := &service.WorkbenchConversation{
		UserID:             userA.ID,
		Title:              "hello",
		Mode:               service.WorkbenchModeChat,
		Endpoint:           service.WorkbenchEndpointChatCompletions,
		Model:              "gpt-5.5",
		LastMessagePreview: "hello",
	}
	require.NoError(t, repo.CreateConversation(ctx, conv))
	require.NotZero(t, conv.ID)

	_, err := repo.GetConversation(ctx, userB.ID, conv.ID)
	require.ErrorIs(t, err, service.ErrWorkbenchConversationNotFound)

	msg := &service.WorkbenchMessage{
		UserID:         userA.ID,
		ConversationID: conv.ID,
		Mode:           service.WorkbenchModeChat,
		Role:           service.WorkbenchRoleUser,
		Content:        "hello",
		Endpoint:       service.WorkbenchEndpointChatCompletions,
		Model:          "gpt-5.5",
		Status:         service.WorkbenchMessageStatusSuccess,
	}
	require.NoError(t, repo.CreateMessage(ctx, msg))

	messages, err := repo.ListMessages(ctx, userA.ID, conv.ID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "hello", messages[0].Content)

	otherMessages, err := repo.ListMessages(ctx, userB.ID, conv.ID)
	require.NoError(t, err)
	require.Empty(t, otherMessages)

	convs, page, err := repo.ListConversations(ctx, userA.ID, pagination.PaginationParams{Page: 1, PageSize: 20}, service.WorkbenchConversationFilters{})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, convs, 1)

	require.NoError(t, repo.SoftDeleteConversation(ctx, userA.ID, conv.ID))
	convs, page, err = repo.ListConversations(ctx, userA.ID, pagination.PaginationParams{Page: 1, PageSize: 20}, service.WorkbenchConversationFilters{})
	require.NoError(t, err)
	require.Zero(t, page.Total)
	require.Empty(t, convs)
}
