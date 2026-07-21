package migrations

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerObservabilityIndexUsesOrderedNonTransactionalMigration(t *testing.T) {
	columns, err := FS.ReadFile("182_openai_scheduler_observability.sql")
	require.NoError(t, err)
	index, err := FS.ReadFile("182_openai_scheduler_observability_index_notx.sql")
	require.NoError(t, err)

	require.NotContains(t, strings.ToUpper(string(columns)), "CREATE INDEX")
	require.Contains(t, strings.ToUpper(string(index)), "CREATE INDEX CONCURRENTLY IF NOT EXISTS")

	files := []string{
		"183_openai_scheduler_phase_two.sql",
		"182_openai_scheduler_observability_index_notx.sql",
		"182_openai_scheduler_observability.sql",
	}
	sort.Strings(files)
	require.Equal(t, []string{
		"182_openai_scheduler_observability.sql",
		"182_openai_scheduler_observability_index_notx.sql",
		"183_openai_scheduler_phase_two.sql",
	}, files)
}

func TestOpenAISchedulerHealthStateMigrationContract(t *testing.T) {
	migration, err := FS.ReadFile("183_openai_scheduler_health_states.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(migration))
	require.Contains(t, sql, "create table if not exists openai_scheduler_health_states")
	require.Contains(t, sql, "id bigserial primary key")
	require.Contains(t, sql, "account_id bigint not null")
	for _, column := range []string{"model_family varchar(100) not null default ''", "endpoint varchar(100) not null default ''"} {
		require.Contains(t, sql, column)
	}
	require.Contains(t, sql, "transport varchar(32) not null default ''")
	require.Contains(t, sql, "state varchar(20) not null default 'running'")
	require.Contains(t, sql, "predicted_ttft_ms decimal(12,3) not null default 0")
	for _, rate := range []string{"error_rate", "rate_limited_rate", "server_error_rate"} {
		require.Contains(t, sql, rate+" decimal(8,4) not null default 0")
	}
	for _, consecutive := range []string{"consecutive_slow", "consecutive_error", "consecutive_success"} {
		require.Contains(t, sql, consecutive+" integer not null default 0")
	}
	for _, sampleCount := range []string{"real_sample_count", "probe_sample_count"} {
		require.Contains(t, sql, sampleCount+" bigint not null default 0")
	}
	for _, nullableTimestamp := range []string{"last_real_at", "last_probe_at", "cooldown_until"} {
		require.Contains(t, sql, nullableTimestamp+" timestamptz")
		require.NotContains(t, sql, nullableTimestamp+" timestamptz not null")
	}
	require.Contains(t, sql, "expires_at timestamptz not null")
	require.Contains(t, sql, "created_at timestamptz not null default now()")
	require.Contains(t, sql, "updated_at timestamptz not null default now()")
	require.Contains(t, sql, "create unique index if not exists idx_openai_scheduler_health_key")
	require.Contains(t, sql, "on openai_scheduler_health_states(account_id, model_family, endpoint, transport)")
	require.Contains(t, sql, "create index if not exists idx_openai_scheduler_health_expiry")
	require.Contains(t, sql, "on openai_scheduler_health_states(expires_at)")
}

func TestOpenAISchedulerDecisionAuditMigrationContract(t *testing.T) {
	migration, err := FS.ReadFile("187_openai_scheduler_decision_audits.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(migration))
	require.Contains(t, sql, "create table if not exists openai_scheduler_decision_audits")
	for _, column := range []string{
		"event_type varchar(40) not null",
		"group_id bigint not null default 0",
		"account_id bigint not null default 0",
		"legacy_account_id bigint not null default 0",
		"confidence varchar(20) not null default ''",
		"eligibility varchar(32) not null default ''",
		"traffic_class varchar(20) not null default ''",
		"created_at timestamptz not null default now()",
	} {
		require.Contains(t, sql, column)
	}
	for _, index := range []string{
		"idx_openai_scheduler_decision_audits_created",
		"idx_openai_scheduler_decision_audits_type_created",
		"idx_openai_scheduler_decision_audits_group_created",
		"idx_openai_scheduler_decision_audits_account_created",
	} {
		require.Contains(t, sql, index)
	}
}
