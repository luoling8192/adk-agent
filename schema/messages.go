package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ChatMessageType is a custom Go type for chat message type (not strictly enforced in DB schema).
type ChatMessageType string

// JoinedChatType is a custom Go type for chat type (not strictly enforced in DB schema).
type JoinedChatType string

type ChatMessage struct {
	ent.Schema
}

// Fields returns the fields of the ChatMessage schema.
func (ChatMessage) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable().Unique(),

		field.String("platform").Default("").NotEmpty(),
		field.String("platform_message_id").Default("").NotEmpty(),
		// field.String("message_type").GoType(ChatMessageType("")).NotEmpty(),

		field.String("from_id").Default("").NotEmpty(),
		field.String("from_name").Default("").NotEmpty(),

		field.UUID("from_user_uuid", uuid.UUID{}).Optional().Nillable(),

		field.UUID("owner_account_id", uuid.UUID{}).Optional().Nillable(),

		field.String("in_chat_id").Default("").NotEmpty(),
		field.String("in_chat_type").Default("").NotEmpty(), // Would be JoinedChatType in types

		field.String("content").Default("").NotEmpty(),

		field.Bool("is_reply").Default(false),
		field.String("reply_to_name").Default("").NotEmpty(),
		field.String("reply_to_id").Default("").NotEmpty(),

		field.Int64("platform_timestamp").Default(0),

		// Vector fields, use []float32 or custom types + annotations as needed for your backend.
		// field.Other("content_vector_1536", []float32{}).Optional().Nillable(),
		// field.Other("content_vector_1024", []float32{}).Optional().Nillable(),
		// field.Other("content_vector_768", []float32{}).Optional().Nillable(),

		field.JSON("jieba_tokens", []string{}).Default([]string{}),
	}
}

func (ChatMessage) Indexes() []ent.Index {
	return []ent.Index{
		// Unique index on (platform, platform_message_id, in_chat_id, owner_account_id)
		index.Fields("platform", "platform_message_id", "in_chat_id", "owner_account_id").Unique(),

		// Vector indices (annotate with DB/PG-specific ops as needed in migration)
		// index.Fields("content_vector_1536"),
		// index.Fields("content_vector_1024"),
		// index.Fields("content_vector_768"),

		// Gin index for jieba_tokens (annotate in SQL/migration scripts as needed)
		index.Fields("jieba_tokens"),

		index.Fields("from_user_uuid"),
	}
}
