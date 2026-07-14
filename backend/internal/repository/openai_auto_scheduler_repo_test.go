package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/openaiautoschedulerscoreevent"
	"github.com/Wei-Shaw/sub2api/ent/openaiautoschedulerscorestate"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newOpenAIAutoSchedulerRepoSQLite(t *testing.T) (*openAIAutoSchedulerRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return &openAIAutoSchedulerRepository{client: client}, client
}

func TestOpenAIAutoSchedulerRepository_UpsertScoreStateConflictsOnScopeAndTrimsModel(t *testing.T) {
	ctx := context.Background()
	repo, client := newOpenAIAutoSchedulerRepoSQLite(t)

	first := service.NewOpenAIAutoSchedulerScoreState(101, 201, "  gpt-5  ")
	first.FinalScore = 4500
	first.BaseScore = 4400
	first.State = service.OpenAIAutoSchedulerStateObserving
	first.RequestCount = 1
	require.NoError(t, repo.UpsertScoreState(ctx, first))

	second := service.NewOpenAIAutoSchedulerScoreState(101, 201, "gpt-5")
	second.FinalScore = 7200
	second.BaseScore = 7100
	second.State = service.OpenAIAutoSchedulerStateRunning
	second.RequestCount = 2
	require.NoError(t, repo.UpsertScoreState(ctx, second))

	count, err := client.OpenAIAutoSchedulerScoreState.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	got, err := repo.GetScoreState(ctx, 101, 201, " gpt-5 ")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "gpt-5", got.Model)
	require.Equal(t, 7200, got.FinalScore)
	require.Equal(t, int64(2), got.RequestCount)

	defaultScope := service.NewOpenAIAutoSchedulerScoreState(101, 201, "   ")
	defaultScope.FinalScore = 6100
	require.NoError(t, repo.UpsertScoreState(ctx, defaultScope))
	defaultScope.FinalScore = 6200
	require.NoError(t, repo.UpsertScoreState(ctx, defaultScope))

	storedDefault, err := repo.GetScoreState(ctx, 101, 201, " ")
	require.NoError(t, err)
	require.NotNil(t, storedDefault)
	require.Equal(t, "", storedDefault.Model)
	require.Equal(t, 6200, storedDefault.FinalScore)

	count, err = client.OpenAIAutoSchedulerScoreState.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestOpenAIAutoSchedulerRepository_UpsertScoreStateClampsBoundedScoresAndPreservesSignedComponents(t *testing.T) {
	ctx := context.Background()
	repo, _ := newOpenAIAutoSchedulerRepoSQLite(t)

	state := service.NewOpenAIAutoSchedulerScoreState(102, 202, "gpt-5")
	state.FinalScore = 12000
	state.BaseScore = -100
	state.LatencyScore = -3500
	state.ErrorScore = -6000
	state.RecoveryScore = 10001
	state.CostScore = -750
	require.NoError(t, repo.UpsertScoreState(ctx, state))

	got, err := repo.GetScoreState(ctx, 102, 202, "gpt-5")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, 10000, got.FinalScore)
	require.Equal(t, 0, got.BaseScore)
	require.Equal(t, -3500, got.LatencyScore)
	require.Equal(t, -6000, got.ErrorScore)
	require.Equal(t, 10000, got.RecoveryScore)
	require.Equal(t, -750, got.CostScore)
}

func TestOpenAIAutoSchedulerRepository_ListScoreStatesOrdersAndCapsPageSize(t *testing.T) {
	ctx := context.Background()
	repo, client := newOpenAIAutoSchedulerRepoSQLite(t)

	base := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 205; i++ {
		state := service.NewOpenAIAutoSchedulerScoreState(int64(1000+i), 300, fmt.Sprintf("model-%03d", i))
		state.FinalScore = 5000
		require.NoError(t, repo.UpsertScoreState(ctx, state))
	}
	require.NoError(t, client.OpenAIAutoSchedulerScoreState.Update().
		Where(openaiautoschedulerscorestate.ModelEQ("model-001")).
		SetFinalScore(9000).
		SetUpdatedAt(base.Add(time.Minute)).
		Exec(ctx))
	require.NoError(t, client.OpenAIAutoSchedulerScoreState.Update().
		Where(openaiautoschedulerscorestate.ModelEQ("model-002")).
		SetFinalScore(9000).
		SetUpdatedAt(base.Add(2*time.Minute)).
		Exec(ctx))
	require.NoError(t, client.OpenAIAutoSchedulerScoreState.Update().
		Where(openaiautoschedulerscorestate.ModelEQ("model-003")).
		SetFinalScore(8500).
		SetUpdatedAt(base.Add(3*time.Minute)).
		Exec(ctx))

	items, total, err := repo.ListScoreStates(ctx, service.OpenAIAutoSchedulerListParams{
		GroupID:  300,
		Page:     1,
		PageSize: 500,
	})
	require.NoError(t, err)
	require.Equal(t, int64(205), total)
	require.Len(t, items, 200)
	require.Equal(t, "model-002", items[0].Model)
	require.Equal(t, "model-001", items[1].Model)
	require.Equal(t, "model-003", items[2].Model)
}

func TestOpenAIAutoSchedulerRepository_HasOpenCircuitScoreStateIgnoresExpiredCooldown(t *testing.T) {
	ctx := context.Background()
	repo, _ := newOpenAIAutoSchedulerRepoSQLite(t)
	now := time.Date(2026, 7, 1, 22, 5, 0, 0, time.UTC)
	expiredCooldown := now.Add(-8 * time.Minute)

	state := service.NewOpenAIAutoSchedulerScoreState(19001, 82, "gpt-5.4")
	state.State = service.OpenAIAutoSchedulerStateOpen
	state.CooldownUntil = &expiredCooldown
	require.NoError(t, repo.UpsertScoreState(ctx, state))

	blocked, err := repo.HasOpenCircuitScoreState(ctx, 19001, 82, "gpt-5.5")
	require.NoError(t, err)
	require.False(t, blocked)

	blocked, err = repo.HasOpenCircuitScoreState(ctx, 19001, 82, "gpt-5.4")

	require.NoError(t, err)
	require.False(t, blocked)
}

func TestOpenAIAutoSchedulerRepository_HasOpenCircuitScoreStateOnlyChecksRequestedModels(t *testing.T) {
	ctx := context.Background()
	repo, _ := newOpenAIAutoSchedulerRepoSQLite(t)
	futureCooldown := time.Now().Add(8 * time.Minute)

	miniState := service.NewOpenAIAutoSchedulerScoreState(19002, 82, "gpt-5.4-mini")
	miniState.State = service.OpenAIAutoSchedulerStateOpen
	miniState.CooldownUntil = &futureCooldown
	require.NoError(t, repo.UpsertScoreState(ctx, miniState))

	blocked, err := repo.HasOpenCircuitScoreState(ctx, 19002, 82, "gpt-5.5")
	require.NoError(t, err)
	require.False(t, blocked)

	primaryState := service.NewOpenAIAutoSchedulerScoreState(19002, 82, "gpt-5.5")
	primaryState.State = service.OpenAIAutoSchedulerStateOpen
	primaryState.CooldownUntil = &futureCooldown
	require.NoError(t, repo.UpsertScoreState(ctx, primaryState))

	blocked, err = repo.HasOpenCircuitScoreState(ctx, 19002, 82, "gpt-5.5")
	require.NoError(t, err)
	require.True(t, blocked)
}

func TestOpenAIAutoSchedulerRepository_InsertScoreEventTruncatesAndPersistsDetails(t *testing.T) {
	ctx := context.Background()
	repo, client := newOpenAIAutoSchedulerRepoSQLite(t)

	createdAt := time.Date(2026, 6, 28, 11, 12, 13, 0, time.UTC)
	latencyMS := 1234
	ttfbMS := 456
	statusCode := 429
	message := "  " + strings.Repeat("x", 1005) + "  "

	require.NoError(t, repo.InsertScoreEvent(ctx, service.OpenAIAutoSchedulerScoreEvent{
		AccountID:   701,
		GroupID:     801,
		Model:       "  gpt-5  ",
		EventType:   service.OpenAIAutoSchedulerEventRateLimited,
		ScoreBefore: -10,
		ScoreAfter:  20000,
		LatencyMS:   &latencyMS,
		TtfbMS:      &ttfbMS,
		StatusCode:  &statusCode,
		Message:     message,
		CreatedAt:   createdAt,
	}))

	got, err := client.OpenAIAutoSchedulerScoreEvent.Query().
		Where(openaiautoschedulerscoreevent.AccountIDEQ(701)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(801), got.GroupID)
	require.Equal(t, "gpt-5", got.Model)
	require.Equal(t, service.OpenAIAutoSchedulerEventRateLimited, got.EventType)
	require.Equal(t, 0, got.ScoreBefore)
	require.Equal(t, 10000, got.ScoreAfter)
	require.NotNil(t, got.LatencyMs)
	require.Equal(t, latencyMS, *got.LatencyMs)
	require.NotNil(t, got.TtfbMs)
	require.Equal(t, ttfbMS, *got.TtfbMs)
	require.NotNil(t, got.StatusCode)
	require.Equal(t, statusCode, *got.StatusCode)
	require.Len(t, got.Message, 1000)
	require.Equal(t, strings.Repeat("x", 1000), got.Message)
	require.WithinDuration(t, createdAt, got.CreatedAt, time.Second)
}

func TestOpenAIAutoSchedulerRepository_InsertScoreEventTruncatesMessageAtRuneBoundary(t *testing.T) {
	ctx := context.Background()
	repo, client := newOpenAIAutoSchedulerRepoSQLite(t)
	message := strings.Repeat("x", 999) + "慢"

	require.NoError(t, repo.InsertScoreEvent(ctx, service.OpenAIAutoSchedulerScoreEvent{
		AccountID: 702,
		GroupID:   802,
		Model:     "gpt-5",
		EventType: service.OpenAIAutoSchedulerEventError,
		Message:   message,
	}))

	got, err := client.OpenAIAutoSchedulerScoreEvent.Query().
		Where(openaiautoschedulerscoreevent.AccountIDEQ(702)).
		Only(ctx)
	require.NoError(t, err)
	require.True(t, utf8.ValidString(got.Message))
	require.LessOrEqual(t, len(got.Message), 1000)
	require.Equal(t, strings.Repeat("x", 999), got.Message)
}

func TestOpenAIAutoSchedulerRepository_InsertScoreEventUsesDefaultTimeWhenOccurredAtIsZero(t *testing.T) {
	ctx := context.Background()
	repo, client := newOpenAIAutoSchedulerRepoSQLite(t)
	before := time.Now()

	require.NoError(t, repo.InsertScoreEvent(ctx, service.OpenAIAutoSchedulerScoreEvent{
		AccountID: 703, GroupID: 803, Model: "gpt-5", EventType: service.OpenAIAutoSchedulerEventSuccess,
	}))

	got, err := client.OpenAIAutoSchedulerScoreEvent.Query().
		Where(openaiautoschedulerscoreevent.AccountIDEQ(703)).
		Only(ctx)
	require.NoError(t, err)
	require.False(t, got.CreatedAt.Before(before))
}

func TestOpenAIAutoSchedulerRepository_ListScoreDailySamplesAggregatesSinceStart(t *testing.T) {
	ctx := context.Background()
	repo, _ := newOpenAIAutoSchedulerRepoSQLite(t)
	since := time.Date(2026, 6, 29, 0, 0, 0, 0, time.Local)
	oldTTFB := 111
	firstTTFB := 222
	lastTTFB := 333

	require.NoError(t, repo.InsertScoreEvent(ctx, service.OpenAIAutoSchedulerScoreEvent{
		AccountID: 1,
		GroupID:   2,
		Model:     "gpt-5",
		EventType: service.OpenAIAutoSchedulerEventProbeSuccess,
		TtfbMS:    &oldTTFB,
		CreatedAt: since.Add(-time.Second),
	}))
	require.NoError(t, repo.InsertScoreEvent(ctx, service.OpenAIAutoSchedulerScoreEvent{
		AccountID: 1,
		GroupID:   2,
		Model:     "gpt-5",
		EventType: service.OpenAIAutoSchedulerEventProbeSuccess,
		TtfbMS:    &firstTTFB,
		CreatedAt: since.Add(time.Hour),
	}))
	require.NoError(t, repo.InsertScoreEvent(ctx, service.OpenAIAutoSchedulerScoreEvent{
		AccountID: 1,
		GroupID:   2,
		Model:     "gpt-5",
		EventType: service.OpenAIAutoSchedulerEventProbeError,
		CreatedAt: since.Add(2 * time.Hour),
	}))
	require.NoError(t, repo.InsertScoreEvent(ctx, service.OpenAIAutoSchedulerScoreEvent{
		AccountID: 1,
		GroupID:   2,
		Model:     "gpt-5",
		EventType: service.OpenAIAutoSchedulerEventSuccess,
		TtfbMS:    &lastTTFB,
		CreatedAt: since.Add(3 * time.Hour),
	}))
	require.NoError(t, repo.InsertScoreEvent(ctx, service.OpenAIAutoSchedulerScoreEvent{
		AccountID: 1,
		GroupID:   3,
		Model:     "gpt-5",
		EventType: service.OpenAIAutoSchedulerEventSuccess,
		TtfbMS:    &lastTTFB,
		CreatedAt: since.Add(4 * time.Hour),
	}))

	samples, err := repo.ListScoreDailySamples(ctx, service.OpenAIAutoSchedulerListParams{GroupID: 2, Model: "gpt-5"}, since)

	require.NoError(t, err)
	require.Len(t, samples, 1)
	require.Equal(t, int64(3), samples[1].RequestCount)
	require.Equal(t, int64(2), samples[1].TtfbSampleCount)
	require.Equal(t, &lastTTFB, samples[1].LastTtfbMS)
}

func TestOpenAIAutoSchedulerRepository_ListScoreEventsFiltersAccount(t *testing.T) {
	ctx := context.Background()
	repo, _ := newOpenAIAutoSchedulerRepoSQLite(t)
	createdAt := time.Date(2026, 7, 14, 1, 2, 3, 0, time.UTC)

	for _, accountID := range []int64{101, 202} {
		require.NoError(t, repo.InsertScoreEvent(ctx, service.OpenAIAutoSchedulerScoreEvent{
			AccountID: accountID,
			GroupID:   20,
			Model:     "gpt-5",
			EventType: service.OpenAIAutoSchedulerEventSuccess,
			CreatedAt: createdAt,
		}))
	}

	events, total, err := repo.ListScoreEvents(ctx, service.OpenAIAutoSchedulerListParams{
		AccountID: 101,
		GroupID:   20,
		Model:     "gpt-5",
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, events, 1)
	require.Equal(t, int64(101), events[0].AccountID)
}

func TestOpenAIAutoSchedulerRepository_ListEnabledOpenAIGroupsFiltersActiveEnabledOpenAI(t *testing.T) {
	ctx := context.Background()
	repo, client := newOpenAIAutoSchedulerRepoSQLite(t)

	mustCreateOpenAIAutoSchedulerRepoGroup(t, ctx, client, "enabled-openai", service.PlatformOpenAI, service.StatusActive, true)
	mustCreateOpenAIAutoSchedulerRepoGroup(t, ctx, client, "disabled-flag", service.PlatformOpenAI, service.StatusActive, false)
	mustCreateOpenAIAutoSchedulerRepoGroup(t, ctx, client, "inactive-openai", service.PlatformOpenAI, service.StatusDisabled, true)
	mustCreateOpenAIAutoSchedulerRepoGroup(t, ctx, client, "anthropic-enabled", service.PlatformAnthropic, service.StatusActive, true)

	groups, err := repo.ListEnabledOpenAIGroups(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "enabled-openai", groups[0].Name)
	require.Equal(t, service.PlatformOpenAI, groups[0].Platform)
	require.Equal(t, service.StatusActive, groups[0].Status)
	require.True(t, groups[0].OpenAIAutoSchedulerEnabled)
}

func mustCreateOpenAIAutoSchedulerRepoGroup(t *testing.T, ctx context.Context, client *dbent.Client, name, platform, status string, enabled bool) {
	t.Helper()

	_, err := client.Group.Create().
		SetName(name).
		SetPlatform(platform).
		SetStatus(status).
		SetOpenaiAutoSchedulerEnabled(enabled).
		Save(ctx)
	require.NoError(t, err)
}
