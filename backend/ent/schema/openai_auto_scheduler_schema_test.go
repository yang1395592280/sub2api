package schema

import (
	"testing"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/entc/load"
	"entgo.io/ent/schema/field"
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

func TestOpenAISchedulerHealthStateFieldContract(t *testing.T) {
	descriptors := map[string]*field.Descriptor{}
	for _, entField := range (OpenAISchedulerHealthState{}).Fields() {
		descriptor := entField.Descriptor()
		descriptors[descriptor.Name] = descriptor
	}

	for _, name := range []string{"consecutive_slow", "consecutive_error", "consecutive_success"} {
		t.Run(name, func(t *testing.T) {
			descriptor, ok := descriptors[name]
			require.True(t, ok, "health schema should include field %s", name)
			require.Equal(t, field.TypeInt, descriptor.Info.Type)
			require.Equal(t, "integer", descriptor.SchemaType[dialect.Postgres])
			require.False(t, descriptor.Optional)
			require.False(t, descriptor.Nillable)
			require.Equal(t, 0, descriptor.Default)
		})
	}

	spec, err := (&load.Config{Path: "."}).Load()
	require.NoError(t, err)

	schemas := map[string]*load.Schema{}
	for _, loadedSchema := range spec.Schemas {
		schemas[loadedSchema.Name] = loadedSchema
	}
	health := requireSchema(t, schemas, "OpenAISchedulerHealthState")

	for _, name := range []string{"last_real_at", "last_probe_at", "cooldown_until"} {
		t.Run(name, func(t *testing.T) {
			schemaField := requireSchemaField(t, health, name)
			require.Equal(t, field.TypeTime, schemaField.Info.Type)
			require.Equal(t, "timestamptz", schemaField.SchemaType[dialect.Postgres])
			require.True(t, schemaField.Optional)
			require.True(t, schemaField.Nillable)
			require.False(t, schemaField.Default)
		})
	}

	for _, tc := range []struct {
		name       string
		hasDefault bool
	}{
		{name: "expires_at", hasDefault: false},
		{name: "created_at", hasDefault: true},
		{name: "updated_at", hasDefault: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schemaField := requireSchemaField(t, health, tc.name)
			require.Equal(t, field.TypeTime, schemaField.Info.Type)
			require.Equal(t, "timestamptz", schemaField.SchemaType[dialect.Postgres])
			require.False(t, schemaField.Optional)
			require.False(t, schemaField.Nillable)
			require.Equal(t, tc.hasDefault, schemaField.Default)
		})
	}
}

func TestOpenAISchedulerHealthStateIndexContract(t *testing.T) {
	spec, err := (&load.Config{Path: "."}).Load()
	require.NoError(t, err)

	schemas := map[string]*load.Schema{}
	for _, loadedSchema := range spec.Schemas {
		schemas[loadedSchema.Name] = loadedSchema
	}
	health := requireSchema(t, schemas, "OpenAISchedulerHealthState")

	uniqueKey := requireSchemaIndex(t, health, "account_id", "model_family", "endpoint", "transport")
	require.Equal(t, "idx_openai_scheduler_health_key", uniqueKey.StorageKey)
	require.True(t, uniqueKey.Unique)

	expiry := requireSchemaIndex(t, health, "expires_at")
	require.Equal(t, "idx_openai_scheduler_health_expiry", expiry.StorageKey)
	require.False(t, expiry.Unique)
}

func requireSchemaIndex(t *testing.T, schema *load.Schema, fields ...string) *load.Index {
	t.Helper()

	for _, schemaIndex := range schema.Indexes {
		require.NotEmpty(t, schemaIndex.Fields, "schema %s index should use fields", schema.Name)
		if len(schemaIndex.Fields) != len(fields) {
			continue
		}
		match := true
		for index := range fields {
			if schemaIndex.Fields[index] != fields[index] {
				match = false
				break
			}
		}
		if match {
			return schemaIndex
		}
	}

	require.Failf(t, "missing schema index", "schema %s should include index on %v", schema.Name, fields)
	return nil
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
		"upstream_price_grouping_enabled",
		"upstream_price_grouping_min",
		"upstream_price_grouping_max",
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
