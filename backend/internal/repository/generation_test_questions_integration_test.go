//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestGenerationQuestionBatchesAndPreferences(t *testing.T) {
	ctx := context.Background()
	repo := NewPelicanTestRepository(integrationDB)
	client := testEntClient(t)
	user, err := client.User.Create().SetEmail(fmt.Sprintf("questions-%d@test.invalid", time.Now().UnixNano())).SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	other, err := client.User.Create().SetEmail(fmt.Sprintf("other-%d@test.invalid", time.Now().UnixNano())).SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	ids := []int64{99000101, 99000102}
	var qids []int64
	t.Cleanup(func() {
		integrationDB.ExecContext(ctx, "DELETE FROM account_pelican_tests WHERE account_id=ANY($1)", pq.Array(ids))
		integrationDB.ExecContext(ctx, "DELETE FROM generation_test_questions WHERE id=ANY($1)", pq.Array(qids))
		client.User.DeleteOneID(user.ID).Exec(ctx)
		client.User.DeleteOneID(other.ID).Exec(ctx)
	})
	for i := 0; i < 3; i++ {
		q, e := repo.SaveQuestion(ctx, service.GenerationTestQuestion{TestKind: "intelligence", Name: fmt.Sprintf("Question %d", i), Prompt: "Return only 42", ExpectedAnswer: "42", Enabled: true})
		require.NoError(t, e)
		qids = append(qids, q.ID)
	}
	first := uuid.NewString()
	rows, err := repo.CreateQuestionBatch(ctx, first, ids, "gpt-5.5", "intelligence", qids[:2], user.ID)
	require.NoError(t, err)
	require.Len(t, rows, 4)
	for _, row := range rows {
		require.Equal(t, "42", row.ExpectedAnswer)
		require.NotEmpty(t, row.QuestionName)
	}
	pref, err := repo.GetTestPreference(ctx, user.ID, "intelligence")
	require.NoError(t, err)
	require.Equal(t, qids[:2], pref.QuestionIDs)
	// An overlapping item must not block a different question for the same account.
	rows, err = repo.CreateQuestionBatch(ctx, uuid.NewString(), ids, "gpt-5.5", "intelligence", qids[1:], user.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	rows, err = repo.CreateQuestionBatch(ctx, uuid.NewString(), ids, "gpt-5.5", "intelligence", qids[:1], user.ID)
	require.NoError(t, err)
	require.Empty(t, rows)
	pref, err = repo.GetTestPreference(ctx, user.ID, "intelligence")
	require.NoError(t, err)
	require.Equal(t, qids[1:], pref.QuestionIDs, "zero enqueue must not replace preference")
	otherPref, err := repo.GetTestPreference(ctx, other.ID, "intelligence")
	require.NoError(t, err)
	require.Empty(t, otherPref.QuestionIDs)
	otherKind, err := repo.GetTestPreference(ctx, user.ID, "pelican")
	require.NoError(t, err)
	require.Empty(t, otherKind.QuestionIDs)
	_, err = repo.SaveQuestion(ctx, service.GenerationTestQuestion{ID: qids[0], TestKind: "intelligence", Name: "Edited", Prompt: "Return 7", ExpectedAnswer: "7", Enabled: false})
	require.NoError(t, err)
	for _, kind := range []string{"intelligence", "pelican"} {
		_, err = repo.CreateQuestionBatch(ctx, uuid.NewString(), ids, "gpt-5.5", kind, qids[:2], user.ID)
		require.ErrorIs(t, err, service.ErrInvalidPelicanBatch)
	}
	history, err := repo.ListBatchHistory(ctx, ids[0], 1, 1, "intelligence")
	require.NoError(t, err)
	require.EqualValues(t, 2, history.Total)
	require.EqualValues(t, 3, history.TotalRecords)
	require.Len(t, history.Items, 1)
	history, err = repo.ListBatchHistory(ctx, ids[0], 2, 1, "intelligence")
	require.NoError(t, err)
	require.Len(t, history.Items, 2)
	for _, row := range history.Items {
		require.Equal(t, "42", row.ExpectedAnswer)
		require.NotEqual(t, "Edited", row.QuestionName)
		require.Empty(t, row.Prompt)
	}
	latest, err := repo.ListLatestBatches(ctx, ids, "intelligence", first)
	require.NoError(t, err)
	require.Len(t, latest, 4)
	latest, err = repo.ListLatestBatches(ctx, ids, "intelligence", "")
	require.NoError(t, err)
	require.Len(t, latest, 2)
	pref, err = repo.GetTestPreference(ctx, user.ID, "intelligence")
	require.NoError(t, err)
	require.Equal(t, qids[1:], pref.QuestionIDs)
	legacy, err := repo.CreateBatch(ctx, uuid.NewString(), ids, "gpt-5.5", "Legacy custom pelican")
	require.NoError(t, err)
	require.Len(t, legacy, 2)
}
func TestGenerationQuestionGroupScheduleIntervalAndSnapshots(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewPelicanTestRepository(integrationDB)
	group, err := client.Group.Create().SetName("question-schedule").SetPlatform("openai").Save(ctx)
	require.NoError(t, err)
	account := mustCreateAccount(t, client, &service.Account{Name: "question-schedule", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true})
	_, err = integrationDB.ExecContext(ctx, "INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)", account.ID, group.ID)
	require.NoError(t, err)
	var qids []int64
	t.Cleanup(func() {
		integrationDB.ExecContext(ctx, "DELETE FROM account_pelican_tests WHERE account_id=$1", account.ID)
		integrationDB.ExecContext(ctx, "DELETE FROM group_pelican_test_schedules WHERE group_id=$1", group.ID)
		integrationDB.ExecContext(ctx, "DELETE FROM generation_test_questions WHERE id=ANY($1)", pq.Array(qids))
		integrationDB.ExecContext(ctx, "DELETE FROM account_groups WHERE account_id=$1", account.ID)
		integrationDB.ExecContext(ctx, "DELETE FROM scheduler_outbox WHERE account_id=$1", account.ID)
		integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id=$1", account.ID)
		integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id=$1", group.ID)
	})
	for i := 0; i < 2; i++ {
		q, e := repo.SaveQuestion(ctx, service.GenerationTestQuestion{TestKind: "pelican", Name: fmt.Sprintf("Scene %d", i), Prompt: "Draw a pelican", Enabled: true})
		require.NoError(t, e)
		qids = append(qids, q.ID)
	}
	cfg := service.GroupPelicanConfig{GroupID: group.ID, TestKind: "pelican", Enabled: true, ModelID: "gpt-5.5", Prompt: "legacy", IntervalMinutes: 45, QuestionIDs: qids}
	require.NoError(t, repo.SaveGroupConfig(ctx, cfg))
	saved, err := repo.GetGroupConfig(ctx, group.ID, "pelican")
	require.NoError(t, err)
	require.Equal(t, 45, saved.IntervalMinutes)
	require.Equal(t, qids, saved.QuestionIDs)
	require.NoError(t, repo.EnqueueDueGroups(ctx))
	latest, err := repo.ListLatestBatches(ctx, []int64{account.ID}, "pelican", "")
	require.NoError(t, err)
	require.Len(t, latest, 2)
	for _, row := range latest {
		enabled, e := repo.ScheduledTestEnabled(ctx, row)
		require.NoError(t, e)
		require.True(t, enabled)
	}
	saved, err = repo.GetGroupConfig(ctx, group.ID, "pelican")
	require.NoError(t, err)
	require.InDelta(t, 45*60, saved.NextRunAt.Sub(*saved.LastRunAt).Seconds(), 1)
	cfg.IntervalMinutes = 60
	require.NoError(t, repo.SaveGroupConfig(ctx, cfg))
	saved, err = repo.GetGroupConfig(ctx, group.ID, "pelican")
	require.NoError(t, err)
	require.InDelta(t, 60*60, saved.NextRunAt.Sub(*saved.LastRunAt).Seconds(), 1)
	_, err = integrationDB.ExecContext(ctx, "UPDATE generation_test_questions SET enabled=false WHERE id=$1", latest[0].QuestionID)
	require.NoError(t, err)
	enabled, err := repo.ScheduledTestEnabled(ctx, latest[0])
	require.NoError(t, err)
	require.False(t, enabled)
	require.ErrorIs(t, repo.SaveGroupConfig(ctx, cfg), service.ErrInvalidPelicanBatch)
	cfg.Enabled = false
	require.NoError(t, repo.SaveGroupConfig(ctx, cfg))
	enabled, err = repo.ScheduledTestEnabled(ctx, latest[1])
	require.NoError(t, err)
	require.False(t, enabled)
}

// Exercise the real upgrade SQL against a pre-254 schema containing live data,
// rather than relying only on a fresh database migration.
func TestGenerationQuestionMigrationPreservesLegacyData(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	schema := "generation_upgrade_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err := tx.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "SET LOCAL search_path TO "+schema)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "CREATE TABLE users(id BIGINT PRIMARY KEY); CREATE TABLE groups(id BIGINT PRIMARY KEY)")
	require.NoError(t, err)
	for _, name := range []string{"241_account_pelican_tests.sql", "242_account_pelican_test_usage.sql", "249_group_pelican_tests.sql", "253_intelligence_tests_and_estimated_cost.sql"} {
		content, e := migrations.FS.ReadFile(name)
		require.NoError(t, e)
		_, e = tx.ExecContext(ctx, string(content))
		require.NoError(t, e)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO groups(id) VALUES(7);
 INSERT INTO group_pelican_test_schedules(group_id,enabled,model_id,prompt,test_kind) VALUES(7,true,'text','Custom legacy pelican','pelican');
 INSERT INTO account_pelican_tests(batch_id,account_id,model_id,prompt,status,test_kind,answer) VALUES
 ('legacy-pelican',11,'text','Custom legacy pelican','queued','pelican',''),
 ('legacy-intelligence',11,'text','Historic candy wording','correct','intelligence','20')`)
	require.NoError(t, err)
	content, err := migrations.FS.ReadFile("254_generation_test_questions.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(content))
	require.NoError(t, err)
	var ids []int64
	var prompt, name, answer, status string
	var id int64
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT question_ids,prompt FROM group_pelican_test_schedules WHERE group_id=7").Scan(pq.Array(&ids), &prompt))
	require.Len(t, ids, 1)
	require.Equal(t, "Custom legacy pelican", prompt)
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT question_id,question_name,status FROM account_pelican_tests WHERE batch_id='legacy-pelican'").Scan(&id, &name, &status))
	require.Equal(t, ids[0], id)
	require.NotEmpty(t, name)
	require.Equal(t, "queued", status)
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT expected_answer,answer FROM account_pelican_tests WHERE batch_id='legacy-intelligence'").Scan(&name, &answer))
	require.Equal(t, "20", name)
	require.Equal(t, "20", answer)
	rollback, err := os.ReadFile("../../../docs/operations/generation-tests-rollback.sql")
	require.NoError(t, err)
	sql := strings.Replace(string(rollback), "BEGIN;", "", 1)
	sql = strings.Replace(sql, "COMMIT;", "", 1)
	_, err = tx.ExecContext(ctx, sql)
	require.NoError(t, err)
	var enabled bool
	var interval int
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT enabled,interval_minutes FROM group_pelican_test_schedules WHERE group_id=7").Scan(&enabled, &interval))
	require.False(t, enabled)
	require.Equal(t, 30, interval)
	var indexDefinition string
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT indexdef FROM pg_indexes WHERE schemaname=$1 AND indexname='idx_account_pelican_tests_active'", schema).Scan(&indexDefinition))
	require.NotContains(t, indexDefinition, "question_id")
}

func TestGenerationQuestionLibraryCapacityKeepsLegacyItemsMaintainable(t *testing.T) {
	ctx := context.Background()
	repo := NewPelicanTestRepository(integrationDB)
	name := "capacity-" + uuid.NewString()
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO generation_test_questions(test_kind,name,prompt,expected_answer)
 SELECT 'intelligence',$1,'Return a number','42' FROM generate_series(1,500)`, name)
	require.NoError(t, err)
	t.Cleanup(func() { integrationDB.ExecContext(ctx, "DELETE FROM generation_test_questions WHERE name=$1", name) })
	_, err = repo.SaveQuestion(ctx, service.GenerationTestQuestion{TestKind: "intelligence", Name: "Over capacity", Prompt: "Return a number", ExpectedAnswer: "42", Enabled: true})
	require.ErrorIs(t, err, service.ErrInvalidPelicanBatch)
	items, err := repo.ListQuestions(ctx, "intelligence")
	require.NoError(t, err)
	require.Greater(t, len(items), 500, "migrated items above the new-create cap must stay accessible")
	var item service.GenerationTestQuestion
	for _, q := range items {
		if q.Name == name {
			item = q
			break
		}
	}
	require.Positive(t, item.ID)
	item.Enabled = false
	item.ExpectedAnswer = "7"
	saved, err := repo.SaveQuestion(ctx, item)
	require.NoError(t, err)
	require.False(t, saved.Enabled)
	require.Equal(t, "7", saved.ExpectedAnswer)
	items, err = repo.ListQuestions(ctx, "intelligence")
	require.NoError(t, err)
	for _, q := range items {
		if q.ID == item.ID {
			require.False(t, q.Enabled)
			require.Equal(t, "7", q.ExpectedAnswer)
		}
	}
}
