package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// MCPTriggerReason indicates why /v1/mcp_servers is triggered.
type MCPTriggerReason string

const (
	MCPTriggerStartupPrefetch    MCPTriggerReason = "startup_prefetch"
	MCPTriggerAuthFailureRecover MCPTriggerReason = "auth_failure_recovery"
	MCPTriggerSessionReset       MCPTriggerReason = "session_reset"
	MCPTriggerMessageAsync       MCPTriggerReason = "message_async"
)

// AccountOutboundContext is the canonical outbound context for one account session.
// All OAuth outbound requests must derive proxy/profile identity from this object.
type AccountOutboundContext struct {
	AccountID           int64
	SessionID           string
	ProxyURL            string
	TokenGeneration     int64
	RuntimeProfileID    string
	TLSProfileID        string
	IdentityFingerprint string
	RequireProxy        bool
}

func ResolveAccountOutboundContext(
	ctx context.Context,
	account *Account,
	proxyRepo ProxyRepository,
) (*AccountOutboundContext, error) {
	if account == nil {
		return nil, fmt.Errorf("account is nil")
	}

	out := &AccountOutboundContext{
		AccountID:        account.ID,
		SessionID:        buildOutboundSessionID(account),
		TokenGeneration:  account.GetCredentialAsInt64("_token_version"),
		RuntimeProfileID: strings.TrimSpace(account.GetCredential("runtime_profile_id")),
		TLSProfileID:     strings.TrimSpace(account.GetCredential("tls_profile_id")),
		RequireProxy:     account.IsAnthropicOAuthOrSetupToken() && account.ProxyID != nil,
	}

	if out.RuntimeProfileID == "" {
		out.RuntimeProfileID = "default"
	}
	if out.TLSProfileID == "" {
		if account.IsTLSFingerprintEnabled() {
			out.TLSProfileID = "claude-code"
		} else {
			out.TLSProfileID = "default"
		}
	}

	// 统一代理来源：优先 account.Proxy，再尝试通过 repo 按 ProxyID 解析。
	if account.Proxy != nil {
		out.ProxyURL = strings.TrimSpace(account.Proxy.URL())
	}
	if out.ProxyURL == "" && account.ProxyID != nil && proxyRepo != nil {
		proxy, err := proxyRepo.GetByID(ctx, *account.ProxyID)
		if err != nil {
			return nil, fmt.Errorf("resolve account proxy: %w", err)
		}
		if proxy != nil {
			out.ProxyURL = strings.TrimSpace(proxy.URL())
		}
	}
	if out.RequireProxy && out.ProxyURL == "" {
		return nil, fmt.Errorf("proxy required for oauth account %d but not available", account.ID)
	}

	out.IdentityFingerprint = buildIdentityFingerprint(account)
	return out, nil
}

func buildOutboundSessionID(account *Account) string {
	if account == nil {
		return "acc-unknown"
	}
	parts := []string{
		fmt.Sprintf("acc:%d", account.ID),
		fmt.Sprintf("tok:%d", account.GetCredentialAsInt64("_token_version")),
		strings.TrimSpace(account.GetCredential("stable_id")),
		strings.TrimSpace(account.GetCredential("machine_id")),
	}
	raw := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(raw))
	return "out-" + hex.EncodeToString(sum[:8])
}

func buildIdentityFingerprint(account *Account) string {
	if account == nil {
		return ""
	}
	parts := []string{
		strings.TrimSpace(account.GetCredential("stable_id")),
		strings.TrimSpace(account.GetCredential("user_id")),
		strings.TrimSpace(account.GetCredential("machine_id")),
		strings.TrimSpace(account.GetCredential("hostname")),
		strings.TrimSpace(account.GetCredential("mac_address")),
		strings.TrimSpace(account.GetCredential("uuid")),
	}
	raw := strings.Join(parts, "|")
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:12])
}
