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

type OpenAIAutoSchedulerScoreState struct{ ent.Schema }

func (OpenAIAutoSchedulerScoreState) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "openai_auto_scheduler_score_states"}}
}

func (OpenAIAutoSchedulerScoreState) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (OpenAIAutoSchedulerScoreState) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Int64("group_id"),
		field.String("model").Default("").MaxLen(200),
		field.Int("final_score").Default(6000),
		field.Int("base_score").Default(6000),
		field.Int("latency_score").Default(0),
		field.Int("error_score").Default(0),
		field.Int("recovery_score").Default(0),
		field.Int("cost_score").Default(0),
		field.String("state").Default("running").MaxLen(20),
		field.Int("consecutive_slow_count").Default(0),
		field.Int("consecutive_error_count").Default(0),
		field.Int("consecutive_success_count").Default(0),
		field.Int64("request_count").Default(0),
		field.Int64("ttfb_sample_count").Default(0),
		field.Float("slow_rate").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}),
		field.Float("error_rate").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}),
		field.Float("stuck_rate").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}),
		field.Time("cooldown_until").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int("last_latency_ms").Optional().Nillable(),
		field.Int("last_ttfb_ms").Optional().Nillable(),
		field.Int("last_status_code").Optional().Nillable(),
		field.String("last_error").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("reason").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("last_checked_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpenAIAutoSchedulerScoreState) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "group_id", "model").Unique(),
		index.Fields("group_id", "final_score"),
		index.Fields("group_id", "state"),
		index.Fields("cooldown_until"),
	}
}
