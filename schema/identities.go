package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
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
			Default("").
			NotEmpty(),

		field.String("platform_user_id").
			Default("").
			NotEmpty(),

		field.String("username").
			Default("").
			NotEmpty(),

		field.String("display_name").
			Default("").
			NotEmpty(),

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
