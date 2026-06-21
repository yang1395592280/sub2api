package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type WorkbenchConversation struct {
	ent.Schema
}

func (WorkbenchConversation) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "workbench_conversations"}}
}

func (WorkbenchConversation) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}

func (WorkbenchConversation) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("title").MaxLen(160).Default("新会话"),
		field.String("mode").MaxLen(16).Default("chat"),
		field.Int64("api_key_id").Optional().Nillable(),
		field.String("endpoint").MaxLen(64).Default("chat_completions"),
		field.String("model").MaxLen(200).Default(""),
		field.String("last_message_preview").MaxLen(300).Default(""),
		field.String("last_error").MaxLen(500).Optional().Nillable(),
		field.Int("message_count").Default(0),
	}
}

func (WorkbenchConversation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("workbench_conversations").Field("user_id").Unique().Required(),
		edge.To("messages", WorkbenchMessage.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (WorkbenchConversation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("user_id", "updated_at"),
		index.Fields("deleted_at"),
	}
}
