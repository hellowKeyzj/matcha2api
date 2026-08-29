package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// GroupShard is a user/account allocation unit under one group.
type GroupShard struct {
	ent.Schema
}

func (GroupShard) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "group_shards"},
	}
}

func (GroupShard) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (GroupShard) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id"),
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		field.String("description").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("status").
			MaxLen(20).
			Default(domain.StatusActive),
	}
}

func (GroupShard) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("group", Group.Type).
			Unique().
			Required().
			Field("group_id"),
		edge.From("users", User.Type).
			Ref("group_shards").
			Through("group_shard_users", GroupShardUser.Type),
		edge.From("accounts", Account.Type).
			Ref("group_shards").
			Through("group_shard_accounts", GroupShardAccount.Type),
	}
}

func (GroupShard) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id"),
		index.Fields("status"),
		index.Fields("group_id", "name").Unique(),
	}
}
