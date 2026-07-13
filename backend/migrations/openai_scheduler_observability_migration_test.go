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
	require.Contains(t, sql, "predicted_ttft_ms decimal(12,3) not null default 0")
	for _, rate := range []string{"error_rate", "rate_limited_rate", "server_error_rate"} {
		require.Contains(t, sql, rate+" decimal(8,4) not null default 0")
	}
	for _, timestamp := range []string{"last_real_at", "last_probe_at", "cooldown_until", "expires_at", "created_at", "updated_at"} {
		require.Contains(t, sql, timestamp+" timestamptz")
	}
	require.Contains(t, sql, "create unique index if not exists idx_openai_scheduler_health_key")
	require.Contains(t, sql, "on openai_scheduler_health_states(account_id, model_family, endpoint, transport)")
	require.Contains(t, sql, "create index if not exists idx_openai_scheduler_health_expiry")
	require.Contains(t, sql, "on openai_scheduler_health_states(expires_at)")
}
