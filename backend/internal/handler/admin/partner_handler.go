package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PartnerHandler handles admin partner management
type PartnerHandler struct {
	partnerService *service.PartnerService
}

// NewPartnerHandler creates a new admin partner handler
func NewPartnerHandler(partnerService *service.PartnerService) *PartnerHandler {
	return &PartnerHandler{
		partnerService: partnerService,
	}
}

// PartnerResponse represents a partner in API responses
type PartnerResponse struct {
	ID                 int64   `json:"id"`
	PartnerName        string  `json:"partner_name"`
	Email              *string `json:"email"`
	Phone              *string `json:"phone"`
	ReferralCode       string  `json:"referral_code"`
	PendingPoints      float64 `json:"pending_points"`
	WithdrawnPoints    float64 `json:"withdrawn_points"`
	Notes              *string `json:"notes"`
	Status             string  `json:"status"`
	SignupBonus        float64 `json:"signup_bonus"`
	MaxPointsPerUser   float64 `json:"max_points_per_user"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	ReferredUsersCount int64   `json:"referred_users_count"`
}

// PartnerCommissionResponse represents a partner commission in API responses
type PartnerCommissionResponse struct {
	ID             int64   `json:"id"`
	PartnerID      int64   `json:"partner_id"`
	ReferredUserID int64   `json:"referred_user_id"`
	Points         float64 `json:"points"`
	SourceCost     float64 `json:"source_cost"`
	CreatedAt      string  `json:"created_at"`
}

func partnerToResponse(p *service.Partner) *PartnerResponse {
	return &PartnerResponse{
		ID:                 p.ID,
		PartnerName:        p.PartnerName,
		Email:              p.Email,
		Phone:              p.Phone,
		ReferralCode:       p.ReferralCode,
		PendingPoints:      p.PendingPoints,
		WithdrawnPoints:    p.WithdrawnPoints,
		Notes:              p.Notes,
		Status:             p.Status,
		SignupBonus:        p.SignupBonus,
		MaxPointsPerUser:   p.MaxPointsPerUser,
		CreatedAt:          p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          p.UpdatedAt.Format(time.RFC3339),
		ReferredUsersCount: p.ReferredUsersCount,
	}
}

func commissionToResponse(c *service.PartnerCommission) *PartnerCommissionResponse {
	return &PartnerCommissionResponse{
		ID:             c.ID,
		PartnerID:      c.PartnerID,
		ReferredUserID: c.ReferredUserID,
		Points:         c.Points,
		SourceCost:     c.SourceCost,
		CreatedAt:      c.CreatedAt.Format(time.RFC3339),
	}
}

// CreatePartnerRequest represents create partner request
type CreatePartnerRequest struct {
	PartnerName      string   `json:"partner_name" binding:"required"`
	Email            *string  `json:"email" binding:"omitempty,email"`
	Phone            *string  `json:"phone"`
	ReferralCode     string   `json:"referral_code"`       // 可选，为空则自动生成
	Notes            *string  `json:"notes"`
	SignupBonus      float64  `json:"signup_bonus"`        // 该伙伴渠道注册赠送余额
	MaxPointsPerUser float64  `json:"max_points_per_user"` // 单用户积分上限，0=无限制
}

// UpdatePartnerRequest represents update partner request
type UpdatePartnerRequest struct {
	PartnerName      *string  `json:"partner_name"`
	Email            *string  `json:"email" binding:"omitempty,email"`
	Phone            *string  `json:"phone"`
	Notes            *string  `json:"notes"`
	Status           *string  `json:"status" binding:"omitempty,oneof=active disabled"`
	SignupBonus      *float64 `json:"signup_bonus"`
	MaxPointsPerUser *float64 `json:"max_points_per_user"`
}

// WithdrawPointsRequest represents withdraw points request
type WithdrawPointsRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

// List handles listing all partners with pagination
// GET /api/v1/admin/partners
func (h *PartnerHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	status := c.Query("status")
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		search = search[:100]
	}

	params := pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}

	partners, paginationResult, err := h.partnerService.List(c.Request.Context(), params, service.PartnerListFilters{
		Status: status,
		Search: search,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]*PartnerResponse, 0, len(partners))
	for i := range partners {
		out = append(out, partnerToResponse(&partners[i]))
	}
	response.Paginated(c, out, paginationResult.Total, page, pageSize)
}

// GetByID handles getting a partner by ID
// GET /api/v1/admin/partners/:id
func (h *PartnerHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid partner ID")
		return
	}

	partner, err := h.partnerService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, partnerToResponse(partner))
}

// Create handles creating a new partner
// POST /api/v1/admin/partners
func (h *PartnerHandler) Create(c *gin.Context) {
	var req CreatePartnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	input := &service.CreatePartnerInput{
		PartnerName:      req.PartnerName,
		Email:            req.Email,
		Phone:            req.Phone,
		ReferralCode:     req.ReferralCode,
		Notes:            req.Notes,
		SignupBonus:      req.SignupBonus,
		MaxPointsPerUser: req.MaxPointsPerUser,
	}

	partner, err := h.partnerService.Create(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, partnerToResponse(partner))
}

// Update handles updating a partner
// PUT /api/v1/admin/partners/:id
func (h *PartnerHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid partner ID")
		return
	}

	var req UpdatePartnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	input := &service.UpdatePartnerInput{
		PartnerName:      req.PartnerName,
		Email:            req.Email,
		Phone:            req.Phone,
		Notes:            req.Notes,
		Status:           req.Status,
		SignupBonus:      req.SignupBonus,
		MaxPointsPerUser: req.MaxPointsPerUser,
	}

	partner, err := h.partnerService.Update(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, partnerToResponse(partner))
}

// Delete handles deleting a partner
// DELETE /api/v1/admin/partners/:id
func (h *PartnerHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid partner ID")
		return
	}

	if err := h.partnerService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Partner deleted"})
}

// WithdrawPoints handles withdrawing points from a partner
// POST /api/v1/admin/partners/:id/withdraw
func (h *PartnerHandler) WithdrawPoints(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid partner ID")
		return
	}

	var req WithdrawPointsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	partner, err := h.partnerService.WithdrawPoints(c.Request.Context(), id, req.Amount)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, partnerToResponse(partner))
}

// ListCommissions handles listing commissions for a partner
// GET /api/v1/admin/partners/:id/commissions
func (h *PartnerHandler) ListCommissions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid partner ID")
		return
	}

	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}

	commissions, paginationResult, err := h.partnerService.ListCommissions(c.Request.Context(), id, params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]*PartnerCommissionResponse, 0, len(commissions))
	for i := range commissions {
		out = append(out, commissionToResponse(&commissions[i]))
	}
	response.Paginated(c, out, paginationResult.Total, page, pageSize)
}
