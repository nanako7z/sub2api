package service

import "time"

// Partner 合作伙伴
type Partner struct {
	ID                 int64
	PartnerName        string
	Email              *string
	Phone              *string
	ReferralCode       string
	PendingPoints      float64
	WithdrawnPoints    float64
	Notes              *string
	Status             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ReferredUsersCount int64 // 已推广用户数（users.partner_id = id 的行数）
}

// PartnerCommission 合作伙伴推广积分记录
type PartnerCommission struct {
	ID             int64
	PartnerID      int64
	ReferredUserID int64
	Points         float64
	SourceCost     float64
	CreatedAt      time.Time
}

// PartnerPointsTrendPoint 合作伙伴积分趋势数据点
type PartnerPointsTrendPoint struct {
	Date            string  `json:"date"`
	TotalPoints     float64 `json:"total_points"`
	TotalSourceCost float64 `json:"total_source_cost"`
	Count           int     `json:"count"`
}

// PartnerPointsLeaderboardItem 合作伙伴积分排行榜项
type PartnerPointsLeaderboardItem struct {
	PartnerID       int64   `json:"partner_id"`
	PartnerName     string  `json:"partner_name"`
	Email           string  `json:"email"`
	TotalPoints     float64 `json:"total_points"`
	TotalSourceCost float64 `json:"total_source_cost"`
	CommissionCount int     `json:"commission_count"`
	ReferredUsers   int     `json:"referred_users"`
}

// PartnerListFilters 合作伙伴列表筛选条件
type PartnerListFilters struct {
	Status string
	Search string // 搜索伙伴名、邮箱、电话
}
