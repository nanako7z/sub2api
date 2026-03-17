package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type partnerRepository struct {
	sql *sql.DB
}

// NewPartnerRepository 创建合作伙伴仓库
func NewPartnerRepository(sqlDB *sql.DB) service.PartnerRepository {
	return &partnerRepository{sql: sqlDB}
}

func (r *partnerRepository) Create(ctx context.Context, partner *service.Partner) error {
	query := `
		INSERT INTO partners (partner_name, email, phone, referral_code, notes, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, updated_at`
	return r.sql.QueryRowContext(ctx, query,
		partner.PartnerName,
		partner.Email,
		partner.Phone,
		partner.ReferralCode,
		partner.Notes,
		partner.Status,
	).Scan(&partner.ID, &partner.CreatedAt, &partner.UpdatedAt)
}

func (r *partnerRepository) GetByID(ctx context.Context, id int64) (*service.Partner, error) {
	query := `SELECT id, partner_name, email, phone, referral_code, pending_points, withdrawn_points, notes, status, created_at, updated_at
		FROM partners WHERE id = $1`
	p := &service.Partner{}
	err := r.sql.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.PartnerName, &p.Email, &p.Phone, &p.ReferralCode,
		&p.PendingPoints, &p.WithdrawnPoints, &p.Notes, &p.Status,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("partner not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get partner by id: %w", err)
	}
	return p, nil
}

func (r *partnerRepository) GetByReferralCode(ctx context.Context, code string) (*service.Partner, error) {
	query := `SELECT id, partner_name, email, phone, referral_code, pending_points, withdrawn_points, notes, status, created_at, updated_at
		FROM partners WHERE UPPER(referral_code) = UPPER($1)`
	p := &service.Partner{}
	err := r.sql.QueryRowContext(ctx, query, code).Scan(
		&p.ID, &p.PartnerName, &p.Email, &p.Phone, &p.ReferralCode,
		&p.PendingPoints, &p.WithdrawnPoints, &p.Notes, &p.Status,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("partner not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get partner by referral code: %w", err)
	}
	return p, nil
}

func (r *partnerRepository) Update(ctx context.Context, partner *service.Partner) error {
	query := `UPDATE partners SET partner_name = $2, email = $3, phone = $4, notes = $5, status = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`
	err := r.sql.QueryRowContext(ctx, query,
		partner.ID,
		partner.PartnerName,
		partner.Email,
		partner.Phone,
		partner.Notes,
		partner.Status,
	).Scan(&partner.UpdatedAt)
	if err == sql.ErrNoRows {
		return fmt.Errorf("partner not found")
	}
	if err != nil {
		return fmt.Errorf("update partner: %w", err)
	}
	return nil
}

func (r *partnerRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.sql.ExecContext(ctx, `DELETE FROM partners WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete partner: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("partner not found")
	}
	return nil
}

// DeleteWithCleanup 删除合作伙伴并清理关联用户的 partner_id。
// 在同一事务中先将引用该 partner 的用户 partner_id 置 NULL，再删除 partner 记录，
// 避免悬空引用导致后续积分静默丢失。
func (r *partnerRepository) DeleteWithCleanup(ctx context.Context, id int64) error {
	tx, err := r.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 清理关联用户的 partner_id
	if _, err := tx.ExecContext(ctx, `UPDATE users SET partner_id = NULL WHERE partner_id = $1`, id); err != nil {
		return fmt.Errorf("clear users partner_id: %w", err)
	}

	// 删除 partner 记录
	result, err := tx.ExecContext(ctx, `DELETE FROM partners WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete partner: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("partner not found")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *partnerRepository) List(ctx context.Context, params pagination.PaginationParams, filters service.PartnerListFilters) ([]service.Partner, *pagination.PaginationResult, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if filters.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filters.Status)
		argIdx++
	}
	if filters.Search != "" {
		searchPattern := "%" + filters.Search + "%"
		conditions = append(conditions, fmt.Sprintf("(partner_name ILIKE $%d OR email ILIKE $%d OR phone ILIKE $%d)", argIdx, argIdx+1, argIdx+2))
		args = append(args, searchPattern, searchPattern, searchPattern)
		argIdx += 3
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM partners %s", where)
	var total int64
	if err := r.sql.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count partners: %w", err)
	}

	pageLimit := params.Limit()
	pages := 0
	if pageLimit > 0 && total > 0 {
		pages = (int(total) + pageLimit - 1) / pageLimit
	}

	// List
	listQuery := fmt.Sprintf(
		`SELECT id, partner_name, email, phone, referral_code, pending_points, withdrawn_points, notes, status, created_at, updated_at
		FROM partners %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, pageLimit, params.Offset())

	rows, err := r.sql.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list partners: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var partners []service.Partner
	for rows.Next() {
		var p service.Partner
		if err := rows.Scan(
			&p.ID, &p.PartnerName, &p.Email, &p.Phone, &p.ReferralCode,
			&p.PendingPoints, &p.WithdrawnPoints, &p.Notes, &p.Status,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan partner: %w", err)
		}
		partners = append(partners, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("list partners rows: %w", err)
	}

	return partners, &pagination.PaginationResult{
		Total:    total,
		Page:     params.Page,
		PageSize: pageLimit,
		Pages:    pages,
	}, nil
}

func (r *partnerRepository) AddPendingPoints(ctx context.Context, partnerID int64, points float64) error {
	result, err := r.sql.ExecContext(ctx,
		`UPDATE partners SET pending_points = pending_points + $2, updated_at = NOW() WHERE id = $1`,
		partnerID, points,
	)
	if err != nil {
		return fmt.Errorf("add pending points: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("partner not found")
	}
	return nil
}

func (r *partnerRepository) WithdrawPoints(ctx context.Context, partnerID int64, amount float64) error {
	result, err := r.sql.ExecContext(ctx,
		`UPDATE partners SET pending_points = pending_points - $2, withdrawn_points = withdrawn_points + $2, updated_at = NOW()
		WHERE id = $1 AND pending_points >= $2`,
		partnerID, amount,
	)
	if err != nil {
		return fmt.Errorf("withdraw points: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("insufficient pending points or partner not found")
	}
	return nil
}

func (r *partnerRepository) CreateCommission(ctx context.Context, commission *service.PartnerCommission) error {
	query := `INSERT INTO partner_commissions (partner_id, referred_user_id, points, source_cost, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`
	now := time.Now()
	commission.CreatedAt = now
	return r.sql.QueryRowContext(ctx, query,
		commission.PartnerID,
		commission.ReferredUserID,
		commission.Points,
		commission.SourceCost,
		now,
	).Scan(&commission.ID)
}

// CreateCommissionAndAddPoints 原子化创建积分记录并增加伙伴待结算积分。
// 两步操作在同一个 sql.Tx 中完成，避免记录入库但积分未增加的不一致问题。
func (r *partnerRepository) CreateCommissionAndAddPoints(ctx context.Context, commission *service.PartnerCommission) error {
	tx, err := r.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 插入积分记录
	now := time.Now()
	commission.CreatedAt = now
	err = tx.QueryRowContext(ctx,
		`INSERT INTO partner_commissions (partner_id, referred_user_id, points, source_cost, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		commission.PartnerID,
		commission.ReferredUserID,
		commission.Points,
		commission.SourceCost,
		now,
	).Scan(&commission.ID)
	if err != nil {
		return fmt.Errorf("insert partner commission: %w", err)
	}

	// 增加伙伴待结算积分
	result, err := tx.ExecContext(ctx,
		`UPDATE partners SET pending_points = pending_points + $2, updated_at = NOW() WHERE id = $1`,
		commission.PartnerID, commission.Points,
	)
	if err != nil {
		return fmt.Errorf("add pending points: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("partner not found")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *partnerRepository) ListCommissions(ctx context.Context, partnerID int64, params pagination.PaginationParams) ([]service.PartnerCommission, *pagination.PaginationResult, error) {
	// Count
	var total int64
	if err := r.sql.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM partner_commissions WHERE partner_id = $1`, partnerID,
	).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count partner commissions: %w", err)
	}

	commPageLimit := params.Limit()
	commPages := 0
	if commPageLimit > 0 && total > 0 {
		commPages = (int(total) + commPageLimit - 1) / commPageLimit
	}

	// List
	rows, err := r.sql.QueryContext(ctx,
		`SELECT id, partner_id, referred_user_id, points, source_cost, created_at
		FROM partner_commissions WHERE partner_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		partnerID, commPageLimit, params.Offset(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list partner commissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var commissions []service.PartnerCommission
	for rows.Next() {
		var c service.PartnerCommission
		if err := rows.Scan(&c.ID, &c.PartnerID, &c.ReferredUserID, &c.Points, &c.SourceCost, &c.CreatedAt); err != nil {
			return nil, nil, fmt.Errorf("scan partner commission: %w", err)
		}
		commissions = append(commissions, c)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("list partner commissions rows: %w", err)
	}

	return commissions, &pagination.PaginationResult{
		Total:    total,
		Page:     params.Page,
		PageSize: commPageLimit,
		Pages:    commPages,
	}, nil
}

// ensure interface compliance
var _ service.PartnerRepository = (*partnerRepository)(nil)
