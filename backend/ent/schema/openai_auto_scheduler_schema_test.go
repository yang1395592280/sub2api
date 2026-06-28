package schema

import (
	"testing"

	"entgo.io/ent/entc/load"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoSchedulerSchemas(t *testing.T) {
	spec, err := (&load.Config{Path: "."}).Load()
	require.NoError(t, err)

	schemas := map[string]*load.Schema{}
	for _, schema := range spec.Schemas {
		schemas[schema.Name] = schema
	}

	group := requireSchema(t, schemas, "Group")
	requireSchemaFields(t, group, "openai_auto_scheduler_enabled")

	state := requireSchema(t, schemas, "OpenAIAutoSchedulerScoreState")
	requireSchemaFields(t, state,
		"account_id", "group_id", "model", "final_score", "base_score",
		"latency_score", "error_score", "recovery_score", "cost_score",
		"state", "consecutive_slow_count", "consecutive_error_count",
		"consecutive_success_count", "request_count", "ttfb_sample_count",
		"slow_rate", "error_rate", "stuck_rate", "cooldown_until",
		"last_latency_ms", "last_ttfb_ms", "last_status_code",
		"last_error", "reason", "last_checked_at")
	requireHasUniqueIndex(t, state, "account_id", "group_id", "model")

	event := requireSchema(t, schemas, "OpenAIAutoSchedulerScoreEvent")
	requireSchemaFields(t, event,
		"account_id", "group_id", "model", "event_type", "score_before",
		"score_after", "latency_ms", "ttfb_ms", "status_code", "message")
}
