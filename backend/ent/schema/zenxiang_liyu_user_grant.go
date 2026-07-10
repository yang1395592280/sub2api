package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ZenxiangLiyuUserGrant stores an individual user's activity access grant.
type ZenxiangLiyuUserGrant struct {
	ent.Schema
}

func (ZenxiangLiyuUserGrant) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "zenxiang_liyu_user_grants"},
	}
}

func (ZenxiangLiyuUserGrant) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (ZenxiangLiyuUserGrant) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Bool("enabled").Default(true),
		field.Int64("granted_by").Optional().Nillable(),
		field.String("notes").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
	}
}

func (ZenxiangLiyuUserGrant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("zenxiang_liyu_grants").Field("user_id").Required().Unique(),
		edge.To("granted_by_user", User.Type).
			Field("granted_by").
			Unique().
			Annotations(entsql.OnDelete(entsql.SetNull)),
	}
}

func (ZenxiangLiyuUserGrant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
		index.Fields("enabled"),
	}
}
