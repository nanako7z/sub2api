package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ReferralHandler 推荐计划相关接口
type ReferralHandler struct {
	referralService *service.ReferralService
}

func NewReferralHandler(referralService *service.ReferralService) *ReferralHandler {
	return &ReferralHandler{referralService: referralService}
}

// GetStats 获取推荐统计信息
// GET /api/v1/referral/stats
func (h *ReferralHandler) GetStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	stats, err := h.referralService.GetStats(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// GenerateCode 生成/获取推荐码
// POST /api/v1/referral/code
func (h *ReferralHandler) GenerateCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	code, err := h.referralService.GetOrCreateReferralCode(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"referral_code": code})
}

// ListReferredUsers 分页被推荐用户列表
// GET /api/v1/referral/users
func (h *ReferralHandler) ListReferredUsers(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}

	users, paginationResult, err := h.referralService.ListReferredUsers(c.Request.Context(), subject.UserID, params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.PaginatedWithResult(c, users, &response.PaginationResult{
		Total:    paginationResult.Total,
		Page:     paginationResult.Page,
		PageSize: paginationResult.PageSize,
		Pages:    paginationResult.Pages,
	})
}

// ListCommissions 分页返佣记录
// GET /api/v1/referral/commissions
func (h *ReferralHandler) ListCommissions(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}

	commissions, paginationResult, err := h.referralService.ListCommissions(c.Request.Context(), subject.UserID, params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.PaginatedWithResult(c, commissions, &response.PaginationResult{
		Total:    paginationResult.Total,
		Page:     paginationResult.Page,
		PageSize: paginationResult.PageSize,
		Pages:    paginationResult.Pages,
	})
}
