package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PlexNotification is an outstanding directory update, not a Plex library item.
type PlexNotification struct{ ent.Schema }

func (PlexNotification) Mixin() []ent.Mixin { return []ent.Mixin{TimeMixin{}} }

func (PlexNotification) Fields() []ent.Field {
	return []ent.Field{
		field.String("path").NotEmpty().Unique(),
		field.Int("revision").Default(1),
		field.Int("attempts").Default(0),
		field.Time("next_attempt_at").Default(time.Now),
		field.String("last_error").Default(""),
	}
}

func (PlexNotification) Indexes() []ent.Index {
	return []ent.Index{index.Fields("next_attempt_at")}
}
