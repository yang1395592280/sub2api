package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type OpenAIAutoSchedulerScoreEvent struct{ ent.Schema }

func (OpenAIAutoSchedulerScoreEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "openai_auto_scheduler_score_events"}}
}

func (OpenAIAutoSchedulerScoreEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Int64("group_id"),
		field.String("model").Default("").MaxLen(200),
		field.String("event_type").MaxLen(40),
		field.Int("score_before").Range(0, 10000),
		field.Int("score_after").Range(0, 10000),
		field.Int("latency_ms").Optional().Nillable(),
		field.Int("ttfb_ms").Optional().Nillable(),
		field.Int("status_code").Optional().Nillable(),
		field.String("message").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").Default(time.Now).Immutable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpenAIAutoSchedulerScoreEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "group_id", "model", "created_at"),
		index.Fields("group_id", "created_at"),
		index.Fields("event_type", "created_at"),
	}
}
