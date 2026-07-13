package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type openAISchedulerHealthQueryLog struct {
	mu      sync.Mutex
	queries []string
	execs   []string
}

func (l *openAISchedulerHealthQueryLog) Log(args ...any) {
	message := fmt.Sprint(args...)
	if !strings.Contains(message, "driver.Query:") &&
		!strings.Contains(message, "driver.Exec:") &&
		!strings.Contains(message, "driver.ExecContext:") {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if strings.Contains(message, "driver.Query:") {
		l.queries = append(l.queries, message)
	} else {
		l.execs = append(l.execs, message)
	}
}

func (l *openAISchedulerHealthQueryLog) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries = nil
	l.execs = nil
}

func (l *openAISchedulerHealthQueryLog) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.queries)
}

func (l *openAISchedulerHealthQueryLog) Only() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.queries) != 1 {
		return ""
	}
	return l.queries[0]
}

func (l *openAISchedulerHealthQueryLog) OnlyStatement() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.queries)+len(l.execs) != 1 {
		return ""
	}
	if len(l.queries) == 1 {
		return l.queries[0]
	}
	return l.execs[0]
}

func newOpenAISchedulerHealthRepoSQLite(t *testing.T) (service.OpenAISchedulerHealthRepository, *dbent.Client, *openAISchedulerHealthQueryLog) {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	queryLog := &openAISchedulerHealthQueryLog{}
	client := enttest.NewClient(t, enttest.WithOptions(
		dbent.Driver(drv),
		dbent.Debug(),
		dbent.Log(queryLog.Log),
	))
	t.Cleanup(func() { _ = client.Close() })
	queryLog.Reset()

	return NewOpenAISchedulerHealthRepository(client), client, queryLog
}

func TestNormalizeUniqueOpenAISchedulerHealthKeys(t *testing.T) {
	keys := []service.OpenAISchedulerHealthKey{
		{AccountID: 10, ModelFamily: " GPT-5 ", Endpoint: " Responses ", Transport: " SSE "},
		{AccountID: 10, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "sse"},
		{AccountID: 11, ModelFamily: " GPT-5 ", Endpoint: " Responses ", Transport: " SSE "},
	}

	require.Equal(t, []service.OpenAISchedulerHealthKey{
		{AccountID: 10, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "sse"},
		{AccountID: 11, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "sse"},
	}, normalizeUniqueOpenAIHealthKeys(keys))
}

func TestOpenAISchedulerHealthRepository_GetBatchNormalizesKeysMapsFieldsAndUsesOneSelect(t *testing.T) {
	ctx := context.Background()
	repo, _, queryLog := newOpenAISchedulerHealthRepoSQLite(t)
	lastRealAt := time.Date(2026, time.July, 13, 10, 11, 12, 0, time.UTC)
	lastProbeAt := lastRealAt.Add(time.Minute)
	cooldownUntil := lastRealAt.Add(2 * time.Minute)
	expiresAt := lastRealAt.Add(-time.Minute)
	snapshot := service.OpenAISchedulerHealthSnapshot{
		Key: service.OpenAISchedulerHealthKey{
			AccountID:   10,
			ModelFamily: " GPT-5 ",
			Endpoint:    " Responses ",
			Transport:   " SSE ",
		},
		State:              service.OpenAIAutoSchedulerStateOpen,
		PredictedTTFTMS:    1400.25,
		ErrorRate:          0.125,
		RateLimitedRate:    0.25,
		ServerErrorRate:    0.375,
		ConsecutiveSlow:    2,
		ConsecutiveError:   3,
		ConsecutiveSuccess: 4,
		RealSampleCount:    5,
		ProbeSampleCount:   6,
		LastRealAt:         &lastRealAt,
		LastProbeAt:        &lastProbeAt,
		CooldownUntil:      &cooldownUntil,
		ExpiresAt:          expiresAt,
	}
	require.NoError(t, repo.Upsert(ctx, snapshot))
	queryLog.Reset()

	normalizedKey := service.OpenAISchedulerHealthKey{
		AccountID:   10,
		ModelFamily: "gpt-5",
		Endpoint:    "responses",
		Transport:   "sse",
	}
	got, err := repo.GetBatch(ctx, []service.OpenAISchedulerHealthKey{
		{AccountID: 10, ModelFamily: " GPT-5 ", Endpoint: " RESPONSES ", Transport: " Sse "},
		normalizedKey,
		{AccountID: 11, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "sse"},
	})
	require.NoError(t, err)
	require.Equal(t, 1, queryLog.Count(), "GetBatch must issue one SELECT for the complete candidate set")
	require.Contains(t, queryLog.Only(), "SELECT")
	require.Contains(t, queryLog.Only(), "openai_scheduler_health_states")
	require.Len(t, got, 1)
	require.NotContains(t, got, service.OpenAISchedulerHealthKey{
		AccountID: 11, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "sse",
	})

	want := snapshot
	want.Key = normalizedKey
	require.Equal(t, want, got[normalizedKey])
	require.True(t, got[normalizedKey].ExpiresAt.Before(lastRealAt), "expired snapshots remain available to policy consumers")
}

func TestOpenAISchedulerHealthRepository_GetBatchEmptyUsesZeroSelects(t *testing.T) {
	ctx := context.Background()
	repo, _, queryLog := newOpenAISchedulerHealthRepoSQLite(t)

	got, err := repo.GetBatch(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
	require.Zero(t, queryLog.Count())
}

func TestOpenAISchedulerHealthRepository_UpsertUpdatesNormalizedUniqueKeyAndClearsNullableTimes(t *testing.T) {
	ctx := context.Background()
	repo, client, queryLog := newOpenAISchedulerHealthRepoSQLite(t)
	lastRealAt := time.Date(2026, time.July, 13, 10, 11, 12, 0, time.UTC)
	lastProbeAt := lastRealAt.Add(time.Minute)
	cooldownUntil := lastRealAt.Add(2 * time.Minute)
	key := service.OpenAISchedulerHealthKey{
		AccountID: 20, ModelFamily: " GPT-5 ", Endpoint: " Responses ", Transport: " SSE ",
	}
	require.NoError(t, repo.Upsert(ctx, service.OpenAISchedulerHealthSnapshot{
		Key: key, State: service.OpenAIAutoSchedulerStateOpen,
		PredictedTTFTMS: 1500, ErrorRate: 0.4, RateLimitedRate: 0.3, ServerErrorRate: 0.2,
		ConsecutiveSlow: 4, ConsecutiveError: 3, ConsecutiveSuccess: 2,
		RealSampleCount: 10, ProbeSampleCount: 11,
		LastRealAt: &lastRealAt, LastProbeAt: &lastProbeAt, CooldownUntil: &cooldownUntil,
		ExpiresAt: cooldownUntil,
	}))

	expiresAt := cooldownUntil.Add(time.Hour)
	queryLog.Reset()
	require.NoError(t, repo.Upsert(ctx, service.OpenAISchedulerHealthSnapshot{
		Key: service.OpenAISchedulerHealthKey{
			AccountID: 20, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "sse",
		},
		State: service.OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500,
		ErrorRate: 0.1, RateLimitedRate: 0.05, ServerErrorRate: 0.025,
		ConsecutiveSlow: 0, ConsecutiveError: 0, ConsecutiveSuccess: 9,
		RealSampleCount: 20, ProbeSampleCount: 21,
		ExpiresAt: expiresAt,
	}))
	require.Contains(t, queryLog.OnlyStatement(), "ON CONFLICT")

	require.Equal(t, 1, client.OpenAISchedulerHealthState.Query().CountX(ctx))
	entity := client.OpenAISchedulerHealthState.Query().OnlyX(ctx)
	require.Equal(t, "gpt-5", entity.ModelFamily)
	require.Equal(t, "responses", entity.Endpoint)
	require.Equal(t, "sse", entity.Transport)
	require.Equal(t, service.OpenAIAutoSchedulerStateRunning, entity.State)
	require.Equal(t, 500.0, entity.PredictedTtftMs)
	require.Equal(t, 0.1, entity.ErrorRate)
	require.Equal(t, 0.05, entity.RateLimitedRate)
	require.Equal(t, 0.025, entity.ServerErrorRate)
	require.Zero(t, entity.ConsecutiveSlow)
	require.Zero(t, entity.ConsecutiveError)
	require.Equal(t, 9, entity.ConsecutiveSuccess)
	require.Equal(t, int64(20), entity.RealSampleCount)
	require.Equal(t, int64(21), entity.ProbeSampleCount)
	require.Nil(t, entity.LastRealAt)
	require.Nil(t, entity.LastProbeAt)
	require.Nil(t, entity.CooldownUntil)
	require.Equal(t, expiresAt, entity.ExpiresAt)
}
