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

// ZenxiangLiyuPrize stores a configurable Zenxiang Liyu prize tier.
type ZenxiangLiyuPrize struct {
	ent.Schema
}

func (ZenxiangLiyuPrize) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "zenxiang_liyu_prizes"},
	}
}

func (ZenxiangLiyuPrize) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (ZenxiangLiyuPrize) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(100).NotEmpty(),
		field.Float("reward_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("probability").SchemaType(map[string]string{dialect.Postgres: "decimal(12,8)"}),
		field.Bool("enabled").Default(true),
		field.Int("sort_order").Default(0),
	}
}

func (ZenxiangLiyuPrize) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "sort_order", "id"),
	}
}
