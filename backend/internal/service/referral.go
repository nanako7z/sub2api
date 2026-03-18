package service

import "time"

// ReferralCommission 推荐返佣记录
type ReferralCommission struct {
	ID             int64
	ReferrerID     int64
	ReferredUserID int64
	Amount         float64
	SourceCost     float64
	CommissionRate float64
	CreatedAt      time.Time
}

// ReferralStats 推荐统计
type ReferralStats struct {
	ReferralCode    string             `json:"referral_code"`
	TotalReferred   int                `json:"total_referred"`
	TotalCommission float64            `json:"total_commission"`
	ReferredUsers   []ReferredUserInfo `json:"referred_users,omitempty"`
	// 当前生效的推荐配置（供前端展示说明）
	SignupBonus          float64 `json:"signup_bonus"`
	CommissionRate       float64 `json:"commission_rate"`
	MaxCommissionPerUser float64 `json:"max_commission_per_user"` // 单用户最大返佣，0=无限制
}

// ReferredUserInfo 被推荐用户信息
type ReferredUserInfo struct {
	UserID          int64     `json:"user_id"`
	Email           string    `json:"email"` // 脱敏
	Username        string    `json:"username"`
	TotalCommission float64   `json:"total_commission"`
	JoinedAt        time.Time `json:"joined_at"`
}

// ReferralCommissionTrendPoint 推荐佣金趋势数据点
type ReferralCommissionTrendPoint struct {
	Date            string  `json:"date"`
	TotalAmount     float64 `json:"total_amount"`
	TotalSourceCost float64 `json:"total_source_cost"`
	Count           int     `json:"count"`
}

// ReferralCommissionLeaderboardItem 推荐佣金排行榜项
type ReferralCommissionLeaderboardItem struct {
	ReferrerID      int64   `json:"referrer_id"`
	Email           string  `json:"email"`
	Username        string  `json:"username"`
	TotalAmount     float64 `json:"total_amount"`
	TotalSourceCost float64 `json:"total_source_cost"`
	CommissionCount int     `json:"commission_count"`
	ReferredUsers   int     `json:"referred_users"`
}

// BalanceSplitResult 拆分扣费结果
type BalanceSplitResult struct {
	NormalDeducted float64
	GiftDeducted   float64
}
