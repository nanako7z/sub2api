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
	SignupBonus    float64 `json:"signup_bonus"`
	CommissionRate float64 `json:"commission_rate"`
}

// ReferredUserInfo 被推荐用户信息
type ReferredUserInfo struct {
	UserID          int64     `json:"user_id"`
	Email           string    `json:"email"` // 脱敏
	Username        string    `json:"username"`
	TotalCommission float64   `json:"total_commission"`
	JoinedAt        time.Time `json:"joined_at"`
}

// BalanceSplitResult 拆分扣费结果
type BalanceSplitResult struct {
	NormalDeducted float64
	GiftDeducted   float64
}
