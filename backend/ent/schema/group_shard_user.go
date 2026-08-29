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

// GroupShardUser holds user membership in a group shard.
type GroupShardUser struct {
	ent.Schema
}

func (GroupShardUser) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "group_shard_users"},
		field.ID("user_id", "group_shard_id"),
	}
}

func (GroupShardUser) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_shard_id"),
		field.Int64("group_id"),
		field.Int64("user_id"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (GroupShardUser) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("group_shard", GroupShard.Type).
			Unique().
			Required().
			Field("group_shard_id"),
		edge.To("group", Group.Type).
			Unique().
			Required().
			Field("group_id"),
		edge.To("user", User.Type).
			Unique().
			Required().
			Field("user_id"),
	}
}

func (GroupShardUser) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id"),
		index.Fields("user_id"),
		index.Fields("group_id", "user_id").Unique(),
	}
}
