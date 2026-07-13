package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type OpenAISchedulerHealthState struct{ ent.Schema }

func (OpenAISchedulerHealthState) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "openai_scheduler_health_states"}}
}

func (OpenAISchedulerHealthState) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (OpenAISchedulerHealthState) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.String("model_family").Default("").MaxLen(100),
		field.String("endpoint").Default("").MaxLen(100),
		field.String("transport").Default("").MaxLen(32),
		field.String("state").Default("running").MaxLen(20),
		field.Float("predicted_ttft_ms").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(12,3)"}),
		field.Float("error_rate").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}),
		field.Float("rate_limited_rate").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}),
		field.Float("server_error_rate").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}),
		field.Int("consecutive_slow").Default(0),
		field.Int("consecutive_error").Default(0),
		field.Int("consecutive_success").Default(0),
		field.Int64("real_sample_count").Default(0),
		field.Int64("probe_sample_count").Default(0),
		field.Time("last_real_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("last_probe_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("cooldown_until").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("expires_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpenAISchedulerHealthState) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "model_family", "endpoint", "transport").Unique(),
		index.Fields("expires_at"),
	}
}
