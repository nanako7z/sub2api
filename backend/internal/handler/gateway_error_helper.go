package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SSEErrorFormat distinguishes SSE error event schemas between platforms.
type SSEErrorFormat int

const (
	// SSEFormatClaude produces: data: {"type":"error","error":{"type":...,"message":...}}
	SSEFormatClaude SSEErrorFormat = iota
	// SSEFormatOpenAI produces: event: error\ndata: {"error":{"type":...,"message":...}}
	SSEFormatOpenAI
)

// GatewayErrorHelper consolidates error-handling methods shared by
// GatewayHandler (Claude) and OpenAIGatewayHandler (OpenAI).
type GatewayErrorHelper struct {
	platform                string
	sseFormat               SSEErrorFormat
	errorPassthroughService *service.ErrorPassthroughService
	usageRecordWorkerPool   *service.UsageRecordWorkerPool
	logComponent            string
}

// errorResponse writes a JSON error response in the platform-specific envelope.
func (h *GatewayErrorHelper) errorResponse(c *gin.Context, status int, errType, message string) {
	if h == nil {
		c.JSON(status, gin.H{"error": gin.H{"type": errType, "message": message}})
		return
	}
	switch h.sseFormat {
	case SSEFormatOpenAI:
		c.JSON(status, gin.H{
			"error": gin.H{
				"type":    errType,
				"message": message,
			},
		})
	default: // SSEFormatClaude
		c.JSON(status, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    errType,
				"message": message,
			},
		})
	}
}

// handleStreamingAwareError sends an error either as an SSE event (if the
// stream has already started) or as a normal JSON response.
func (h *GatewayErrorHelper) handleStreamingAwareError(c *gin.Context, status int, errType, message string, streamStarted bool) {
	if streamStarted {
		flusher, ok := c.Writer.(http.Flusher)
		if ok {
			var errorEvent string
			switch h.sseFormat {
			case SSEFormatOpenAI:
				// event: error\ndata: {"error":{"type":...,"message":...}}\n\n
				errorEvent = "event: error\ndata: " +
					`{"error":{"type":` + strconv.Quote(errType) + `,"message":` + strconv.Quote(message) + `}}` + "\n\n"
			default: // SSEFormatClaude
				// data: {"type":"error","error":{"type":...,"message":...}}\n\n
				errorEvent = `data: {"type":"error","error":{"type":` + strconv.Quote(errType) + `,"message":` + strconv.Quote(message) + `}}` + "\n\n"
			}
			if _, err := fmt.Fprint(c.Writer, errorEvent); err != nil {
				_ = c.Error(err)
			}
			flusher.Flush()
		}
		return
	}

	h.errorResponse(c, status, errType, message)
}

// ensureForwardErrorResponse writes a fallback error response when Forward
// returned an error but nothing has been written to the client yet.
func (h *GatewayErrorHelper) ensureForwardErrorResponse(c *gin.Context, streamStarted bool) bool {
	if c == nil || c.Writer == nil || c.Writer.Written() {
		return false
	}
	h.handleStreamingAwareError(c, http.StatusBadGateway, "upstream_error", "Upstream request failed", streamStarted)
	return true
}

// handleConcurrencyError sends a 429 response for concurrency slot failures.
func (h *GatewayErrorHelper) handleConcurrencyError(c *gin.Context, _ error, slotType string, streamStarted bool) {
	h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error",
		fmt.Sprintf("Concurrency limit exceeded for %s, please retry later", slotType), streamStarted)
}

// mapUpstreamError maps an upstream HTTP status code to the downstream status,
// error type, and human-readable message.
func (h *GatewayErrorHelper) mapUpstreamError(statusCode int) (int, string, string) {
	switch statusCode {
	case 401:
		return http.StatusBadGateway, "upstream_error", "Upstream authentication failed, please contact administrator"
	case 403:
		return http.StatusBadGateway, "upstream_error", "Upstream access forbidden, please contact administrator"
	case 429:
		return http.StatusTooManyRequests, "rate_limit_error", "Upstream rate limit exceeded, please retry later"
	case 529:
		// Claude uses "overloaded_error"; OpenAI uses "upstream_error".
		errType := "overloaded_error"
		if h.sseFormat == SSEFormatOpenAI {
			errType = "upstream_error"
		}
		return http.StatusServiceUnavailable, errType, "Upstream service overloaded, please retry later"
	case 500, 502, 503, 504:
		return http.StatusBadGateway, "upstream_error", "Upstream service temporarily unavailable"
	default:
		return http.StatusBadGateway, "upstream_error", "Upstream request failed"
	}
}

// handleFailoverExhausted processes an UpstreamFailoverError, checking
// passthrough rules first and falling back to the default error mapping.
func (h *GatewayErrorHelper) handleFailoverExhausted(c *gin.Context, failoverErr *service.UpstreamFailoverError, platform string, streamStarted bool) {
	statusCode := failoverErr.StatusCode
	responseBody := failoverErr.ResponseBody

	// 先检查透传规则
	if h.errorPassthroughService != nil && len(responseBody) > 0 {
		if rule := h.errorPassthroughService.MatchRule(platform, statusCode, responseBody); rule != nil {
			respCode := statusCode
			if !rule.PassthroughCode && rule.ResponseCode != nil {
				respCode = *rule.ResponseCode
			}

			msg := service.ExtractUpstreamErrorMessage(responseBody)
			if !rule.PassthroughBody && rule.CustomMessage != nil {
				msg = *rule.CustomMessage
			}

			if rule.SkipMonitoring {
				c.Set(service.OpsSkipPassthroughKey, true)
			}

			h.handleStreamingAwareError(c, respCode, "upstream_error", msg, streamStarted)
			return
		}
	}

	status, errType, errMsg := h.mapUpstreamError(statusCode)
	h.handleStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

// handleFailoverExhaustedSimple is a simplified version for cases without a
// response body, using only the status code for error mapping.
func (h *GatewayErrorHelper) handleFailoverExhaustedSimple(c *gin.Context, statusCode int, streamStarted bool) {
	status, errType, errMsg := h.mapUpstreamError(statusCode)
	h.handleStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

// submitUsageRecordTask submits a usage record task to the worker pool, falling
// back to synchronous execution if the pool is not available.
func (h *GatewayErrorHelper) submitUsageRecordTask(task service.UsageRecordTask) {
	if task == nil {
		return
	}
	if h.usageRecordWorkerPool != nil {
		h.usageRecordWorkerPool.Submit(task)
		return
	}
	// 回退路径：worker 池未注入时同步执行，避免退回到无界 goroutine 模式。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", h.logComponent),
				zap.Any("panic", recovered),
			).Error("gateway.usage_record_task_panic_recovered")
		}
	}()
	task(ctx)
}
