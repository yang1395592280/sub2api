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

type WindsurfAccount struct {
	ent.Schema
}

func (WindsurfAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "windsurf_accounts"},
	}
}

func (WindsurfAccount) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (WindsurfAccount) Fields() []ent.Field {
	return []ent.Field{
		field.String("account").
			NotEmpty().
			MaxLen(255).
			Unique(),
		field.String("password_encrypted").
			NotEmpty().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Bool("enabled").
			Default(false),
		field.Int64("maintained_by"),
		field.Time("maintained_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("status_updated_by").
			Optional().
			Nillable(),
		field.Time("status_updated_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (WindsurfAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled"),
		index.Fields("maintained_at"),
	}
}
