package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// ZenxiangLiyuSetting stores the singleton Zenxiang Liyu activity settings.
type ZenxiangLiyuSetting struct {
	ent.Schema
}

func (ZenxiangLiyuSetting) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "zenxiang_liyu_settings",
			Checks: map[string]string{
				"zenxiang_liyu_settings_singleton": "id = 1",
			},
		},
	}
}

func (ZenxiangLiyuSetting) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (ZenxiangLiyuSetting) Fields() []ent.Field {
	return []ent.Field{
		field.Bool("global_enabled").Default(false),
		field.Float("ticket_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(2),
		field.Float("minimum_balance").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(10),
		field.Int("daily_play_limit").Default(5),
		field.Float("ticket_usage_threshold").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(5),
		field.Int("daily_ticket_limit").Default(3),
		field.Float("unit_sale_price").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(0.1),
		field.Float("unit_cost_price").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(0.05),
	}
}
