package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// JoinedChatType enumerates the supported chat types in Go for strong typing.
type JoinedChatType string

const (
	JoinedChatTypeUser    JoinedChatType = "user"
	JoinedChatTypeChannel JoinedChatType = "channel"
	JoinedChatTypeGroup   JoinedChatType = "group"
)

// JoinedChat defines the Ent schema for the joined_chats table.
type JoinedChat struct {
	ent.Schema
}

// Fields provides the schema definition for the joined_chats table columns.
func (JoinedChat) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			Unique(),

		field.String("platform").
			Default("").
			NotEmpty(),

		field.String("chat_id").
			Default("").
			NotEmpty(),

		field.String("chat_name").
			Default("").
			NotEmpty(),

		field.String("chat_type").
			Default(string(JoinedChatTypeUser)).
			NotEmpty().
			Annotations(), // Could add enum check in validator

		field.Int64("dialog_date").
			Default(0),

		field.Int64("created_at").
			DefaultFunc(func() int64 { return time.Now().UnixMilli() }),

		field.Int64("updated_at").
			DefaultFunc(func() int64 { return time.Now().UnixMilli() }).
			UpdateDefault(func() int64 { return time.Now().UnixMilli() }),
	}
}

// Indexes defines unique and other indexes for joined_chats.
func (JoinedChat) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("platform", "chat_id").
			Unique().
			StorageKey("platform_chat_id_unique_index"),
	}
}
