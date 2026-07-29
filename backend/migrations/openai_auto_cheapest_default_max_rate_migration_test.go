package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoCheapestDefaultMaxRateMigration(t *testing.T) {
	content, err := FS.ReadFile("193_openai_auto_cheapest_default_max_rate.sql")
	require.NoError(t, err)

	migration := strings.ToLower(string(content))
	require.Contains(t, migration, "set openai_auto_group_max_rate_multiplier = 0.2")
	require.Contains(t, migration, "group_select_mode = 'openai_auto_cheapest'")
	require.Contains(t, migration, "coalesce(openai_auto_group_max_rate_multiplier, 0) <= 0")
	require.Contains(t, migration, "deleted_at is null")
}
