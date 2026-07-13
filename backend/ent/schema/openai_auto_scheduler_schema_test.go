package schema

import (
	"testing"

	"entgo.io/ent"
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
	requireBasisPointValidators(t, OpenAIAutoSchedulerScoreState{}.Fields(),
		"final_score", "base_score")
	requireNoBasisPointValidators(t, OpenAIAutoSchedulerScoreState{}.Fields(),
		"latency_score", "error_score", "recovery_score", "cost_score")

	event := requireSchema(t, schemas, "OpenAIAutoSchedulerScoreEvent")
	requireSchemaFields(t, event,
		"account_id", "group_id", "model", "event_type", "score_before",
		"score_after", "latency_ms", "ttfb_ms", "status_code", "message")
	requireBasisPointValidators(t, OpenAIAutoSchedulerScoreEvent{}.Fields(),
		"score_before", "score_after")
}

func TestOpenAISchedulerHealthStateSchema(t *testing.T) {
	spec, err := (&load.Config{Path: "."}).Load()
	require.NoError(t, err)

	schemas := map[string]*load.Schema{}
	for _, schema := range spec.Schemas {
		schemas[schema.Name] = schema
	}

	health := requireSchema(t, schemas, "OpenAISchedulerHealthState")
	requireSchemaFields(t, health,
		"account_id", "model_family", "endpoint", "transport", "state",
		"predicted_ttft_ms", "error_rate", "rate_limited_rate", "server_error_rate",
		"consecutive_slow", "consecutive_error", "consecutive_success",
		"real_sample_count", "probe_sample_count", "last_real_at", "last_probe_at",
		"cooldown_until", "expires_at",
	)
	requireHasUniqueIndex(t, health, "account_id", "model_family", "endpoint", "transport")
}

func TestGroupUpstreamPriceGuardSchemaFields(t *testing.T) {
	spec, err := (&load.Config{Path: "."}).Load()
	require.NoError(t, err)

	schemas := map[string]*load.Schema{}
	for _, schema := range spec.Schemas {
		schemas[schema.Name] = schema
	}

	group := requireSchema(t, schemas, "Group")
	requireSchemaFields(t, group,
		"upstream_balance_refresh_enabled",
		"upstream_balance_refresh_interval_seconds",
		"upstream_price_max_multiplier",
	)
}

func requireNoBasisPointValidators(t *testing.T, fields []ent.Field, names ...string) {
	t.Helper()

	for _, name := range names {
		for _, entField := range fields {
			descriptor := entField.Descriptor()
			if descriptor.Name == name {
				require.Empty(t, descriptor.Validators, "field %s should allow signed scheduler components", name)
				break
			}
		}
	}
}

func requireBasisPointValidators(t *testing.T, fields []ent.Field, names ...string) {
	t.Helper()

	for _, name := range names {
		validator := requireIntFieldValidator(t, fields, name)
		require.NoError(t, validator(0), "field %s should allow the lower basis-point bound", name)
		require.NoError(t, validator(10000), "field %s should allow the upper basis-point bound", name)
		require.Error(t, validator(-1), "field %s should reject negative basis points", name)
		require.Error(t, validator(10001), "field %s should reject basis points above 10000", name)
	}
}

func requireIntFieldValidator(t *testing.T, fields []ent.Field, name string) func(int) error {
	t.Helper()

	for _, entField := range fields {
		descriptor := entField.Descriptor()
		if descriptor.Name != name {
			continue
		}
		require.NotEmpty(t, descriptor.Validators, "field %s should include a validator", name)
		validator, ok := descriptor.Validators[0].(func(int) error)
		require.True(t, ok, "field %s validator should be func(int) error", name)
		return validator
	}

	require.Failf(t, "missing field validator", "schema should include field %s", name)
	return nil
}
