package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// PartnerRepository 合作伙伴仓库接口
type PartnerRepository interface {
	Create(ctx context.Context, partner *Partner) error
	GetByID(ctx context.Context, id int64) (*Partner, error)
	GetByReferralCode(ctx context.Context, code string) (*Partner, error)
	Update(ctx context.Context, partner *Partner) error
	Delete(ctx context.Context, id int64) error
	DeleteWithCleanup(ctx context.Context, id int64) error
	List(ctx context.Context, params pagination.PaginationParams, filters PartnerListFilters) ([]Partner, *pagination.PaginationResult, error)

	// 积分操作（原子 SQL 更新）
	AddPendingPoints(ctx context.Context, partnerID int64, points float64) error
	WithdrawPoints(ctx context.Context, partnerID int64, amount float64) error

	// 积分记录
	CreateCommission(ctx context.Context, commission *PartnerCommission) error
	CreateCommissionAndAddPoints(ctx context.Context, commission *PartnerCommission) error
	ListCommissions(ctx context.Context, partnerID int64, params pagination.PaginationParams) ([]PartnerCommission, *pagination.PaginationResult, error)

	// Dashboard analytics
	GetPointsTrend(ctx context.Context, start, end time.Time) ([]PartnerPointsTrendPoint, error)
	GetPointsLeaderboard(ctx context.Context, start, end time.Time, limit int) ([]PartnerPointsLeaderboardItem, error)
}
