package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ZenxiangLiyuRecord stores an immutable Zenxiang Liyu play ledger row.
type ZenxiangLiyuRecord struct {
	ent.Schema
}

func (ZenxiangLiyuRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "zenxiang_liyu_records"},
	}
}

func (ZenxiangLiyuRecord) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_id").MaxLen(128),
		field.Int64("user_id"),
		field.Time("play_date").SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.Float("ticket_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("reward_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("user_net_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("system_revenue").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("system_expense").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("system_profit").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Int64("prize_id").Optional().Nillable(),
		field.String("prize_name_snapshot").MaxLen(100),
		field.Float("probability_snapshot").SchemaType(map[string]string{dialect.Postgres: "decimal(12,8)"}),
		field.JSON("config_snapshot", json.RawMessage{}).Default(func() json.RawMessage { return json.RawMessage("{}") }).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Float("balance_before").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("balance_after_ticket").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("balance_after_reward").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ZenxiangLiyuRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("zenxiang_liyu_records").Field("user_id").Required().Unique(),
		edge.From("prize", ZenxiangLiyuPrize.Type).Ref("records").Field("prize_id").Unique(),
	}
}

func (ZenxiangLiyuRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "request_id").Unique(),
		index.Fields("user_id", "play_date"),
		index.Fields("play_date"),
		index.Fields("prize_id"),
	}
}
