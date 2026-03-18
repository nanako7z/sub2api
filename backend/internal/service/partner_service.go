package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// PartnerService 合作伙伴服务
type PartnerService struct {
	partnerRepo    PartnerRepository
	userRepo       UserRepository
	promoRepo      PromoCodeRepository
	settingService *SettingService
}

// NewPartnerService 创建合作伙伴服务
func NewPartnerService(
	partnerRepo PartnerRepository,
	userRepo UserRepository,
	promoRepo PromoCodeRepository,
	settingService *SettingService,
) *PartnerService {
	return &PartnerService{
		partnerRepo:    partnerRepo,
		userRepo:       userRepo,
		promoRepo:      promoRepo,
		settingService: settingService,
	}
}

// CreatePartnerInput 创建合作伙伴输入
type CreatePartnerInput struct {
	PartnerName      string
	Email            *string
	Phone            *string
	ReferralCode     string  // 可选，为空则自动生成（带 P 前缀）
	Notes            *string
	SignupBonus      float64 // 注册赠送余额（创建时必填）
	MaxPointsPerUser float64 // 单个被推荐用户可获取的最大积分，0=无限制
}

// UpdatePartnerInput 更新合作伙伴输入
type UpdatePartnerInput struct {
	PartnerName      *string
	Email            *string
	Phone            *string
	Notes            *string
	Status           *string
	SignupBonus      *float64
	MaxPointsPerUser *float64
}

// Create 创建合作伙伴
func (s *PartnerService) Create(ctx context.Context, input *CreatePartnerInput) (*Partner, error) {
	// 验证至少一个联系方式
	hasEmail := input.Email != nil && *input.Email != ""
	hasPhone := input.Phone != nil && *input.Phone != ""
	if !hasEmail && !hasPhone {
		return nil, fmt.Errorf("at least one contact method (email or phone) is required")
	}

	var code string
	if input.ReferralCode != "" {
		// 管理员手动指定推荐码，检查冲突
		code = strings.ToUpper(input.ReferralCode)
		if err := s.checkCodeConflicts(ctx, code); err != nil {
			return nil, err
		}
	} else {
		// 自动生成带 P 前缀的推荐码
		var err error
		code, err = s.generateUniqueCode(ctx)
		if err != nil {
			return nil, err
		}
	}

	partner := &Partner{
		PartnerName:      input.PartnerName,
		Email:            input.Email,
		Phone:            input.Phone,
		ReferralCode:     code,
		Notes:            input.Notes,
		Status:           StatusActive,
		SignupBonus:      input.SignupBonus,
		MaxPointsPerUser: input.MaxPointsPerUser,
	}

	if err := s.partnerRepo.Create(ctx, partner); err != nil {
		return nil, fmt.Errorf("create partner: %w", err)
	}
	return partner, nil
}

// Update 更新合作伙伴
func (s *PartnerService) Update(ctx context.Context, id int64, input *UpdatePartnerInput) (*Partner, error) {
	partner, err := s.partnerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.PartnerName != nil {
		partner.PartnerName = *input.PartnerName
	}
	if input.Email != nil {
		partner.Email = input.Email
	}
	if input.Phone != nil {
		partner.Phone = input.Phone
	}
	if input.Notes != nil {
		partner.Notes = input.Notes
	}
	if input.Status != nil {
		partner.Status = *input.Status
	}
	if input.SignupBonus != nil {
		partner.SignupBonus = *input.SignupBonus
	}
	if input.MaxPointsPerUser != nil {
		partner.MaxPointsPerUser = *input.MaxPointsPerUser
	}

	// 验证至少一个联系方式
	hasEmail := partner.Email != nil && *partner.Email != ""
	hasPhone := partner.Phone != nil && *partner.Phone != ""
	if !hasEmail && !hasPhone {
		return nil, fmt.Errorf("at least one contact method (email or phone) is required")
	}

	if err := s.partnerRepo.Update(ctx, partner); err != nil {
		return nil, err
	}
	return partner, nil
}

// Delete 删除合作伙伴（同时清理关联用户的 partner_id）
func (s *PartnerService) Delete(ctx context.Context, id int64) error {
	return s.partnerRepo.DeleteWithCleanup(ctx, id)
}

// GetByID 获取合作伙伴
func (s *PartnerService) GetByID(ctx context.Context, id int64) (*Partner, error) {
	return s.partnerRepo.GetByID(ctx, id)
}

// List 列出合作伙伴
func (s *PartnerService) List(ctx context.Context, params pagination.PaginationParams, filters PartnerListFilters) ([]Partner, *pagination.PaginationResult, error) {
	return s.partnerRepo.List(ctx, params, filters)
}

// ValidatePartnerReferralCode 验证伙伴推荐码是否有效
func (s *PartnerService) ValidatePartnerReferralCode(ctx context.Context, code string) (*Partner, error) {
	if !s.settingService.IsPartnerEnabled(ctx) {
		return nil, fmt.Errorf("partner system is disabled")
	}
	partner, err := s.partnerRepo.GetByReferralCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if partner.Status != StatusActive {
		return nil, fmt.Errorf("partner is not active")
	}
	return partner, nil
}

// RecordCommission 记录推广积分（在被推荐用户消费普通余额时调用）
// 比例固定 1:1（$1 消费 = 1 积分）
func (s *PartnerService) RecordCommission(ctx context.Context, referredUserID int64, normalBalanceCost float64) {
	if normalBalanceCost <= 0 {
		return
	}

	if !s.settingService.IsPartnerEnabled(ctx) {
		return
	}

	// 获取用户的 partner_id
	user, err := s.userRepo.GetByID(ctx, referredUserID)
	if err != nil || user.PartnerID == nil {
		return
	}
	partnerID := *user.PartnerID

	// 检查合作伙伴是否仍处于活跃状态，已禁用的伙伴不再计入积分
	partner, err := s.partnerRepo.GetByID(ctx, partnerID)
	if err != nil || partner.Status != StatusActive {
		return
	}

	// 1:1 积分比例
	points := normalBalanceCost

	commission := &PartnerCommission{
		PartnerID:      partnerID,
		ReferredUserID: referredUserID,
		Points:         points,
		SourceCost:     normalBalanceCost,
	}

	// 原子化创建积分记录 + 增加待结算积分 + cap 检查，在同一事务中完成
	maxPerUser := partner.MaxPointsPerUser
	actualPoints, err := s.partnerRepo.CreateCommissionWithCapAndAddPoints(ctx, commission, maxPerUser)
	if err != nil {
		slog.Error("failed to create partner commission and add points", "partner_id", partnerID, "points", points, "error", err)
		return
	}
	if actualPoints <= 0 {
		return // 已达上限
	}
}

// WithdrawPoints 提现推广积分（管理员操作）
func (s *PartnerService) WithdrawPoints(ctx context.Context, partnerID int64, amount float64) (*Partner, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("withdraw amount must be positive")
	}

	if err := s.partnerRepo.WithdrawPoints(ctx, partnerID, amount); err != nil {
		return nil, err
	}

	return s.partnerRepo.GetByID(ctx, partnerID)
}

// ListCommissions 列出伙伴积分记录
func (s *PartnerService) ListCommissions(ctx context.Context, partnerID int64, params pagination.PaginationParams) ([]PartnerCommission, *pagination.PaginationResult, error) {
	return s.partnerRepo.ListCommissions(ctx, partnerID, params)
}

// checkCodeConflicts 检查推荐码是否与其他码冲突
func (s *PartnerService) checkCodeConflicts(ctx context.Context, code string) error {
	// 检查用户推荐码
	if s.userRepo != nil {
		if _, err := s.userRepo.GetByReferralCode(ctx, code); err == nil {
			return fmt.Errorf("referral code conflicts with existing user referral code")
		}
	}
	// 检查优惠码
	if s.promoRepo != nil {
		if _, err := s.promoRepo.GetByCode(ctx, code); err == nil {
			return fmt.Errorf("referral code conflicts with existing promo code")
		}
	}
	// 检查其他伙伴推荐码
	if _, err := s.partnerRepo.GetByReferralCode(ctx, code); err == nil {
		return fmt.Errorf("referral code conflicts with existing partner referral code")
	}
	return nil
}

// generateUniqueCode 生成唯一的伙伴推荐码（P + 7位随机字符）
func (s *PartnerService) generateUniqueCode(ctx context.Context) (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const suffixLen = 7

	for attempts := 0; attempts < 50; attempts++ {
		suffix := make([]byte, suffixLen)
		for i := range suffix {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return "", fmt.Errorf("crypto/rand: %w", err)
			}
			suffix[i] = charset[n.Int64()]
		}
		code := "P" + string(suffix)

		if err := s.checkCodeConflicts(ctx, code); err != nil {
			slog.Warn("partner code conflict, retrying", "code", code, "attempt", attempts+1)
			continue
		}
		return code, nil
	}
	return "", fmt.Errorf("failed to generate unique partner referral code after retries")
}
