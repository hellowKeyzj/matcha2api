package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// GroupSharedAccount makes a parent-group account eligible for every shard.
type GroupSharedAccount struct {
	ent.Schema
}

func (GroupSharedAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "group_shared_accounts"},
		field.ID("account_id", "group_id"),
	}
}

func (GroupSharedAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id"),
		field.Int64("account_id"),
		field.String("platform").MaxLen(50),
		field.Int("priority").Default(50),
		field.Time("created_at").Immutable().Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (GroupSharedAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("group", Group.Type).Unique().Required().Field("group_id").
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("account", Account.Type).Unique().Required().Field("account_id").
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (GroupSharedAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id"),
		index.Fields("group_id", "platform"),
	}
}
