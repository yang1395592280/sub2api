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
