package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Identity defines the Ent schema for the identities table.
type Identity struct {
	ent.Schema
}

// Fields provides the schema definition for the identities table columns.
func (Identity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			Unique(),

		field.String("platform").
			Default(""),

		field.String("platform_user_id").
			Default(""),

		field.String("username").
			Default(""),

		field.String("display_name").
			Default(""),

		field.String("profile_photo_url").
			Default(""),

		field.Strings("alt_ids").
			Optional(),

		field.Int64("created_at").
			DefaultFunc(func() int64 { return time.Now().UnixMilli() }),

		field.Int64("updated_at").
			DefaultFunc(func() int64 { return time.Now().UnixMilli() }).
			UpdateDefault(func() int64 { return time.Now().UnixMilli() }),
	}
}

// Edges defines the many-to-many relation between Identities and Events.
func (Identity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("events", Event.Type),
	}
}

func (Identity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("platform", "platform_user_id").Unique(),
	}
}
