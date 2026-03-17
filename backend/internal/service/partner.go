package service

import "time"

// Partner 合作伙伴
type Partner struct {
	ID              int64
	PartnerName     string
	Email           *string
	Phone           *string
	ReferralCode    string
	PendingPoints   float64
	WithdrawnPoints float64
	Notes           *string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
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

// PartnerListFilters 合作伙伴列表筛选条件
type PartnerListFilters struct {
	Status string
	Search string // 搜索伙伴名、邮箱、电话
}
