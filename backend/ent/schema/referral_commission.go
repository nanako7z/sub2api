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

// ReferralCommission holds the schema definition for the ReferralCommission entity.
//
// 推荐返佣记录：记录推荐人因被推荐用户消费普通余额而获得的赠送余额
//
// 删除策略：硬删除
type ReferralCommission struct {
	ent.Schema
}

func (ReferralCommission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "referral_commissions"},
	}
}

func (ReferralCommission) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("referrer_id").
			Comment("获得返佣的推荐人"),
		field.Int64("referred_user_id").
			Comment("产生消费的被推荐用户"),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0).
			Comment("返佣金额"),
		field.Float("source_cost").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0).
			Comment("触发返佣的原始消费金额"),
		field.Float("commission_rate").
			Default(0).
			Comment("记录时的返佣比例"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ReferralCommission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("referrer", User.Type).
			Ref("referral_commissions").
			Field("referrer_id").
			Unique().
			Required(),
	}
}

func (ReferralCommission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("referrer_id", "created_at"),
		index.Fields("referred_user_id"),
		index.Fields("referrer_id", "referred_user_id"),
	}
}
