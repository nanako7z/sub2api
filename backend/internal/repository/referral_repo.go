package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/referralcommission"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ReferralRepository 推荐返佣数据访问接口
type ReferralRepository interface {
	CreateCommission(ctx context.Context, commission *service.ReferralCommission) error
	CreateCommissionWithCap(ctx context.Context, commission *service.ReferralCommission, maxPerUser float64) (float64, error)
	GetTotalCommissionForPair(ctx context.Context, referrerID, referredUserID int64) (float64, error)
	GetTotalCommission(ctx context.Context, referrerID int64) (float64, error)
	ListCommissions(ctx context.Context, referrerID int64, params pagination.PaginationParams) ([]service.ReferralCommission, *pagination.PaginationResult, error)
	ListReferredUsers(ctx context.Context, referrerID int64, params pagination.PaginationParams) ([]service.ReferredUserInfo, *pagination.PaginationResult, error)
	CountReferredUsers(ctx context.Context, referrerID int64) (int, error)
}

type referralRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewReferralRepository(client *dbent.Client, sqlDB *sql.DB) ReferralRepository {
	return &referralRepository{client: client, sql: sqlDB}
}

// CreateCommission 创建一条返佣记录
func (r *referralRepository) CreateCommission(ctx context.Context, commission *service.ReferralCommission) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ReferralCommission.Create().
		SetReferrerID(commission.ReferrerID).
		SetReferredUserID(commission.ReferredUserID).
		SetAmount(commission.Amount).
		SetSourceCost(commission.SourceCost).
		SetCommissionRate(commission.CommissionRate).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create referral commission: %w", err)
	}
	return nil
}

// GetTotalCommissionForPair 获取推荐人从某个被推荐用户获得的返佣总额（用于 cap 检查）
func (r *referralRepository) GetTotalCommissionForPair(ctx context.Context, referrerID, referredUserID int64) (float64, error) {
	var total float64
	rows, err := r.sql.QueryContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM referral_commissions WHERE referrer_id = $1 AND referred_user_id = $2`,
		referrerID, referredUserID,
	)
	if err != nil {
		return 0, fmt.Errorf("get total commission for pair: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.Scan(&total); err != nil {
			return 0, fmt.Errorf("scan total commission: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("get total commission for pair rows: %w", err)
	}
	return total, nil
}

// GetTotalCommission 获取推荐人的返佣总额
func (r *referralRepository) GetTotalCommission(ctx context.Context, referrerID int64) (float64, error) {
	var total float64
	rows, err := r.sql.QueryContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM referral_commissions WHERE referrer_id = $1`,
		referrerID,
	)
	if err != nil {
		return 0, fmt.Errorf("get total commission: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.Scan(&total); err != nil {
			return 0, fmt.Errorf("scan total commission: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("get total commission rows: %w", err)
	}
	return total, nil
}

// ListCommissions 分页列出推荐人的返佣记录
func (r *referralRepository) ListCommissions(ctx context.Context, referrerID int64, params pagination.PaginationParams) ([]service.ReferralCommission, *pagination.PaginationResult, error) {
	client := clientFromContext(ctx, r.client)

	query := client.ReferralCommission.Query().
		Where(referralcommission.ReferrerIDEQ(referrerID))

	total, err := query.Count(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("count referral commissions: %w", err)
	}

	items, err := query.
		Order(dbent.Desc(referralcommission.FieldCreatedAt)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list referral commissions: %w", err)
	}

	commissions := make([]service.ReferralCommission, 0, len(items))
	for _, item := range items {
		commissions = append(commissions, service.ReferralCommission{
			ID:             item.ID,
			ReferrerID:     item.ReferrerID,
			ReferredUserID: item.ReferredUserID,
			Amount:         item.Amount,
			SourceCost:     item.SourceCost,
			CommissionRate: item.CommissionRate,
			CreatedAt:      item.CreatedAt,
		})
	}

	return commissions, &pagination.PaginationResult{Total: int64(total), Page: params.Page, PageSize: params.PageSize}, nil
}

// ListReferredUsers 分页列出被推荐用户信息（带总返佣）
func (r *referralRepository) ListReferredUsers(ctx context.Context, referrerID int64, params pagination.PaginationParams) ([]service.ReferredUserInfo, *pagination.PaginationResult, error) {
	client := clientFromContext(ctx, r.client)

	query := client.User.Query().
		Where(
			dbuser.ReferrerIDEQ(referrerID),
			dbuser.DeletedAtIsNil(),
		)

	total, err := query.Count(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("count referred users: %w", err)
	}

	users, err := query.
		Order(dbent.Desc(dbuser.FieldCreatedAt)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list referred users: %w", err)
	}

	// 批量查询所有被推荐用户的返佣总额，避免 N+1
	commissionMap := make(map[int64]float64)
	if len(users) > 0 {
		userIDs := make([]int64, len(users))
		for i, u := range users {
			userIDs[i] = u.ID
		}
		commissionMap, _ = r.batchGetCommissionsForPairs(ctx, referrerID, userIDs)
	}

	result := make([]service.ReferredUserInfo, 0, len(users))
	for _, u := range users {
		result = append(result, service.ReferredUserInfo{
			UserID:          u.ID,
			Email:           maskEmail(u.Email),
			Username:        u.Username,
			TotalCommission: commissionMap[u.ID],
			JoinedAt:        u.CreatedAt,
		})
	}

	return result, &pagination.PaginationResult{Total: int64(total), Page: params.Page, PageSize: params.PageSize}, nil
}

// batchGetCommissionsForPairs 批量获取推荐人对多个被推荐用户的返佣总额
func (r *referralRepository) batchGetCommissionsForPairs(ctx context.Context, referrerID int64, referredUserIDs []int64) (map[int64]float64, error) {
	if len(referredUserIDs) == 0 {
		return nil, nil
	}

	// 构建 IN 子句的占位符
	placeholders := make([]string, len(referredUserIDs))
	args := make([]any, 0, len(referredUserIDs)+1)
	args = append(args, referrerID)
	for i, id := range referredUserIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, id)
	}

	query := fmt.Sprintf(
		`SELECT referred_user_id, COALESCE(SUM(amount), 0)
		 FROM referral_commissions
		 WHERE referrer_id = $1 AND referred_user_id IN (%s)
		 GROUP BY referred_user_id`,
		strings.Join(placeholders, ","),
	)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch get commissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64]float64)
	for rows.Next() {
		var userID int64
		var total float64
		if err := rows.Scan(&userID, &total); err != nil {
			return nil, fmt.Errorf("scan batch commission: %w", err)
		}
		result[userID] = total
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("batch get commissions rows: %w", err)
	}
	return result, nil
}

// CountReferredUsers 统计被推荐用户数
func (r *referralRepository) CountReferredUsers(ctx context.Context, referrerID int64) (int, error) {
	client := clientFromContext(ctx, r.client)
	count, err := client.User.Query().
		Where(
			dbuser.ReferrerIDEQ(referrerID),
			dbuser.DeletedAtIsNil(),
		).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count referred users: %w", err)
	}
	return count, nil
}

// maskEmail 脱敏邮箱 user@example.com -> u***@example.com
func maskEmail(email string) string {
	idx := -1
	for i := len(email) - 1; i >= 0; i-- {
		if email[i] == '@' {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return "***"
	}
	return string(email[0]) + "***" + email[idx:]
}

// CreateCommissionWithCap 原子化创建返佣记录并检查 cap
// 如果 maxPerUser > 0，则原子地检查当前总额 + 新金额是否超过上限，自动截断。
// 返回实际记录的金额（0 表示已达上限未插入）。
func (r *referralRepository) CreateCommissionWithCap(ctx context.Context, commission *service.ReferralCommission, maxPerUser float64) (float64, error) {
	if maxPerUser <= 0 {
		// 无上限，直接插入
		if err := r.CreateCommission(ctx, commission); err != nil {
			return 0, err
		}
		return commission.Amount, nil
	}

	// 原子化：在单条 SQL 中检查 cap 并插入，用 FOR UPDATE 锁防止并发超额
	query := `
		WITH current AS (
			SELECT COALESCE(SUM(amount), 0) AS total
			FROM referral_commissions
			WHERE referrer_id = $1 AND referred_user_id = $2
			FOR UPDATE
		),
		capped AS (
			SELECT LEAST($3, GREATEST($6 - total, 0)) AS final_amount
			FROM current
		)
		INSERT INTO referral_commissions (referrer_id, referred_user_id, amount, source_cost, commission_rate, created_at)
		SELECT $1, $2, final_amount, $4, $5, NOW()
		FROM capped
		WHERE final_amount > 0
		RETURNING amount
	`
	rows, err := r.sql.QueryContext(ctx, query,
		commission.ReferrerID,     // $1
		commission.ReferredUserID, // $2
		commission.Amount,         // $3
		commission.SourceCost,     // $4
		commission.CommissionRate, // $5
		maxPerUser,                // $6
	)
	if err != nil {
		return 0, fmt.Errorf("create commission with cap: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var actualAmount float64
	if rows.Next() {
		if err := rows.Scan(&actualAmount); err != nil {
			return 0, fmt.Errorf("scan commission amount: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("create commission with cap rows: %w", err)
	}
	return actualAmount, nil
}

// ensure interface compliance
var _ ReferralRepository = (*referralRepository)(nil)
