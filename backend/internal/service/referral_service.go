package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// ReferralServiceDeps 推荐服务依赖（避免循环依赖）
type ReferralServiceDeps struct {
	UserRepo       UserRepository
	ReferralRepo   ReferralRepositoryInterface
	SettingService *SettingService
	BillingCache   BillingCacheInvalidator
	PromoRepo      PromoCodeRepository
	PartnerRepo    PartnerRepository
	EntClient      *dbent.Client // 用于事务支持
}

// BillingCacheInvalidator 计费缓存失效接口
type BillingCacheInvalidator interface {
	InvalidateUserBalance(userID int64)
}

// ReferralRepositoryInterface 推荐仓库接口（在 service 层定义以避免 import cycle）
type ReferralRepositoryInterface interface {
	CreateCommission(ctx context.Context, commission *ReferralCommission) error
	CreateCommissionWithCap(ctx context.Context, commission *ReferralCommission, maxPerUser float64) (float64, error)
	CreateCommissionWithCapAndCredit(ctx context.Context, commission *ReferralCommission, maxPerUser float64, referrerID int64) (float64, error)
	GetTotalCommissionForPair(ctx context.Context, referrerID, referredUserID int64) (float64, error)
	GetTotalCommission(ctx context.Context, referrerID int64) (float64, error)
	ListCommissions(ctx context.Context, referrerID int64, params pagination.PaginationParams) ([]ReferralCommission, *pagination.PaginationResult, error)
	ListReferredUsers(ctx context.Context, referrerID int64, params pagination.PaginationParams) ([]ReferredUserInfo, *pagination.PaginationResult, error)
	CountReferredUsers(ctx context.Context, referrerID int64) (int, error)

	// Dashboard analytics
	GetCommissionTrend(ctx context.Context, start, end time.Time) ([]ReferralCommissionTrendPoint, error)
	GetCommissionLeaderboard(ctx context.Context, start, end time.Time, limit int) ([]ReferralCommissionLeaderboardItem, error)
}

// ReferralService 推荐系统服务
type ReferralService struct {
	userRepo       UserRepository
	referralRepo   ReferralRepositoryInterface
	settingService *SettingService
	billingCache   BillingCacheInvalidator
	promoRepo      PromoCodeRepository
	partnerRepo    PartnerRepository
	entClient      *dbent.Client
}

func NewReferralService(deps ReferralServiceDeps) *ReferralService {
	return &ReferralService{
		userRepo:       deps.UserRepo,
		referralRepo:   deps.ReferralRepo,
		settingService: deps.SettingService,
		billingCache:   deps.BillingCache,
		promoRepo:      deps.PromoRepo,
		partnerRepo:    deps.PartnerRepo,
		entClient:      deps.EntClient,
	}
}

// GetOrCreateReferralCode 获取或生成用户的推荐码
func (s *ReferralService) GetOrCreateReferralCode(ctx context.Context, userID int64) (string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}

	if user.ReferralCode != nil && *user.ReferralCode != "" {
		return *user.ReferralCode, nil
	}

	// 生成推荐码，如果因唯一约束冲突或与优惠码重复则重试
	for attempts := 0; attempts < 50; attempts++ {
		code, err := s.generateCode()
		if err != nil {
			return "", fmt.Errorf("generate referral code: %w", err)
		}
		// 检查是否与已有优惠码冲突（优惠码使用 case-insensitive 查询）
		if s.promoRepo != nil {
			if _, err := s.promoRepo.GetByCode(ctx, code); err == nil {
				slog.Warn("referral code conflicts with promo code, retrying", "code", code, "attempt", attempts+1)
				continue
			}
		}
		// 检查是否与已有伙伴推荐码冲突
		if s.partnerRepo != nil {
			if _, err := s.partnerRepo.GetByReferralCode(ctx, code); err == nil {
				slog.Warn("referral code conflicts with partner code, retrying", "code", code, "attempt", attempts+1)
				continue
			}
		}
		if err := s.userRepo.SetReferralCode(ctx, userID, code); err != nil {
			slog.Warn("referral code conflict, retrying", "code", code, "attempt", attempts+1, "error", err)
			continue
		}
		return code, nil
	}

	return "", fmt.Errorf("failed to generate unique referral code after retries")
}

// ValidateReferralCode 验证推荐码是否有效
func (s *ReferralService) ValidateReferralCode(ctx context.Context, code string) (*User, error) {
	if !s.settingService.IsReferralEnabled(ctx) {
		return nil, fmt.Errorf("referral system is disabled")
	}
	user, err := s.userRepo.GetByReferralCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if user.Status != StatusActive {
		return nil, fmt.Errorf("referrer account is not active")
	}
	return user, nil
}

// ApplyReferralSignup 注册时应用推荐关系
// 所有操作在同一个 ent 事务中执行，避免部分成功导致状态不一致
func (s *ReferralService) ApplyReferralSignup(ctx context.Context, newUserID int64, referrerID int64) error {
	// 防止自己推荐自己
	if newUserID == referrerID {
		return fmt.Errorf("cannot refer yourself")
	}

	// 在事务中执行所有操作
	if s.entClient != nil {
		tx, err := s.entClient.Tx(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		txCtx := dbent.NewTxContext(ctx, tx)

		if err := s.applyReferralSignupInCtx(txCtx, newUserID, referrerID); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}
	} else {
		// 无 ent client 时直接执行（向后兼容测试场景）
		if err := s.applyReferralSignupInCtx(ctx, newUserID, referrerID); err != nil {
			return err
		}
	}

	// 失效推荐人的计费缓存（事务提交后执行）
	referrerBonus := s.settingService.GetReferralReferrerBonus(ctx)
	if referrerBonus > 0 && s.billingCache != nil {
		s.billingCache.InvalidateUserBalance(referrerID)
	}

	return nil
}

// applyReferralSignupInCtx 在给定 context（可能包含事务）中执行推荐注册逻辑
func (s *ReferralService) applyReferralSignupInCtx(ctx context.Context, newUserID int64, referrerID int64) error {
	// 设置推荐关系
	if err := s.userRepo.SetReferrer(ctx, newUserID, referrerID); err != nil {
		return fmt.Errorf("set referrer: %w", err)
	}

	// 给新用户发放注册赠送余额
	signupBonus := s.settingService.GetReferralSignupBonus(ctx)
	if signupBonus > 0 {
		if err := s.userRepo.UpdateGiftBalance(ctx, newUserID, signupBonus); err != nil {
			return fmt.Errorf("give signup bonus to user %d: %w", newUserID, err)
		}
	}

	// 给推荐人发放即时奖励
	referrerBonus := s.settingService.GetReferralReferrerBonus(ctx)
	if referrerBonus > 0 {
		if err := s.userRepo.UpdateGiftBalance(ctx, referrerID, referrerBonus); err != nil {
			return fmt.Errorf("give referrer bonus to user %d: %w", referrerID, err)
		}
	}

	return nil
}

// RecordCommission 记录返佣（在被推荐用户消费普通余额时调用）
func (s *ReferralService) RecordCommission(ctx context.Context, referredUserID int64, normalBalanceCost float64) {
	if normalBalanceCost <= 0 {
		return
	}

	if !s.settingService.IsReferralEnabled(ctx) {
		return
	}

	// 获取被推荐用户的推荐人
	user, err := s.userRepo.GetByID(ctx, referredUserID)
	if err != nil || user.ReferrerID == nil {
		return
	}
	referrerID := *user.ReferrerID

	// 获取返佣比例
	rate := s.settingService.GetReferralCommissionRate(ctx)
	if rate <= 0 {
		return
	}

	// 计算返佣金额
	commissionAmount := normalBalanceCost * rate / 100
	if commissionAmount <= 0 {
		return
	}

	commission := &ReferralCommission{
		ReferrerID:     referrerID,
		ReferredUserID: referredUserID,
		Amount:         commissionAmount,
		SourceCost:     normalBalanceCost,
		CommissionRate: rate,
	}

	// 原子化 cap 检查 + 插入 + 推荐人余额增加，在同一事务中完成
	maxPerUser := s.settingService.GetReferralMaxCommissionPerUser(ctx)
	actualAmount, err := s.referralRepo.CreateCommissionWithCapAndCredit(ctx, commission, maxPerUser, referrerID)
	if err != nil {
		slog.Error("failed to create referral commission", "error", err)
		return
	}
	if actualAmount <= 0 {
		return // 已达上限
	}

	// 失效推荐人的计费缓存
	if s.billingCache != nil {
		s.billingCache.InvalidateUserBalance(referrerID)
	}
}

// GetStats 获取推荐统计信息
func (s *ReferralService) GetStats(ctx context.Context, userID int64) (*ReferralStats, error) {
	code, err := s.GetOrCreateReferralCode(ctx, userID)
	if err != nil {
		return nil, err
	}

	totalReferred, err := s.referralRepo.CountReferredUsers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count referred users: %w", err)
	}

	totalCommission, err := s.referralRepo.GetTotalCommission(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get total commission: %w", err)
	}

	return &ReferralStats{
		ReferralCode:    code,
		TotalReferred:   totalReferred,
		TotalCommission: totalCommission,
		SignupBonus:     s.settingService.GetReferralSignupBonus(ctx),
		CommissionRate:  s.settingService.GetReferralCommissionRate(ctx),
	}, nil
}

// ListReferredUsers 分页列出被推荐用户
func (s *ReferralService) ListReferredUsers(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ReferredUserInfo, *pagination.PaginationResult, error) {
	return s.referralRepo.ListReferredUsers(ctx, userID, params)
}

// ListCommissions 分页列出返佣记录
func (s *ReferralService) ListCommissions(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ReferralCommission, *pagination.PaginationResult, error) {
	return s.referralRepo.ListCommissions(ctx, userID, params)
}

// generateCode 生成随机推荐码（唯一性由 DB 唯一约束 + 上层重试保证）
func (s *ReferralService) generateCode() (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 排除容易混淆的 I/O/0/1
	const codeLen = 8

	code := make([]byte, codeLen)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("crypto/rand: %w", err)
		}
		code[i] = charset[n.Int64()]
	}
	return string(code), nil
}
