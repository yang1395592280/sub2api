package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type WorkbenchMessage struct {
	ent.Schema
}

func (WorkbenchMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "workbench_messages"}}
}

func (WorkbenchMessage) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}

func (WorkbenchMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("conversation_id"),
		field.Int64("user_id"),
		field.String("mode").MaxLen(16),
		field.String("role").MaxLen(16),
		field.String("content").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.Int64("api_key_id").Optional().Nillable(),
		field.String("endpoint").MaxLen(64).Default(""),
		field.String("model").MaxLen(200).Default(""),
		field.JSON("request_options", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("response_metadata", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("image_outputs", []domain.WorkbenchImageOutput{}).
			Default(func() []domain.WorkbenchImageOutput { return []domain.WorkbenchImageOutput{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("status").MaxLen(16).Default("success"),
		field.String("error_message").MaxLen(500).Optional().Nillable(),
	}
}

func (WorkbenchMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("conversation", WorkbenchConversation.Type).Ref("messages").Field("conversation_id").Unique().Required(),
		edge.From("user", User.Type).Ref("workbench_messages").Field("user_id").Unique().Required(),
	}
}

func (WorkbenchMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("conversation_id", "created_at"),
		index.Fields("user_id", "created_at"),
		index.Fields("status"),
		index.Fields("deleted_at"),
	}
}
