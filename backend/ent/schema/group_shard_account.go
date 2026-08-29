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

// GroupShardAccount holds account bindings for one group shard.
type GroupShardAccount struct {
	ent.Schema
}

func (GroupShardAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "group_shard_accounts"},
		field.ID("account_id", "group_shard_id"),
	}
}

func (GroupShardAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_shard_id"),
		field.Int64("group_id"),
		field.Int64("account_id"),
		field.String("platform").
			MaxLen(50),
		field.Int("priority").
			Default(50),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (GroupShardAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("group_shard", GroupShard.Type).
			Unique().
			Required().
			Field("group_shard_id"),
		edge.To("group", Group.Type).
			Unique().
			Required().
			Field("group_id"),
		edge.To("account", Account.Type).
			Unique().
			Required().
			Field("account_id"),
	}
}

func (GroupShardAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id"),
		index.Fields("account_id"),
		index.Fields("group_id", "account_id").Unique(),
		index.Fields("group_id", "platform"),
		index.Fields("group_shard_id", "platform"),
		index.Fields("priority"),
	}
}
