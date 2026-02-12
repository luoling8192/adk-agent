package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Event defines the Ent schema for the events table.
type Event struct {
	ent.Schema
}

// Fields provides the schema definition for the events table columns.
func (Event) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			Unique(),

		field.String("platform").
			Default("").
			NotEmpty(),

		field.String("name").
			Default("").
			NotEmpty(),

		field.Strings("tags").
			Optional(),

		field.String("description").
			Default("").
			NotEmpty(),

		field.String("from_name").
			Default("").
			NotEmpty(),

		field.String("in_chat_id").
			Default("").
			NotEmpty(),

		field.String("in_chat_type").
			Default("").
			NotEmpty(),

		field.Int64("platform_timestamp").
			Default(0),

		field.JSON("evidence_message_ids", []uuid.UUID{}).
			Default([]uuid.UUID{}),

		field.Int64("created_at").
			DefaultFunc(func() int64 { return time.Now().UnixMilli() }),

		field.Int64("updated_at").
			DefaultFunc(func() int64 { return time.Now().UnixMilli() }).
			UpdateDefault(func() int64 { return time.Now().UnixMilli() }),
	}
}

// Edges defines the many-to-many relation between Events and Identities.
func (Event) Edges() []ent.Edge {
	return []ent.Edge{
		// Many-to-many relation: events <-> identities
		// This could be e.g. tagged or participants, adjust "identities" as needed.
		// This assumes there is also an Events edge in the Identity schema for symmetry.
		edge.From("identities", Identity.Type).
			Ref("events"),
	}
}
