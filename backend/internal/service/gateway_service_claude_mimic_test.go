package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	gocache "github.com/patrickmn/go-cache"
)

func TestApplyClaudeCodeMimicHeaders_MessagesStream(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/messages", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	applyClaudeCodeMimicHeaders(req, true, claude.EndpointMessages)

	if req.Header.Get("x-stainless-helper-method") != "stream" {
		t.Fatalf("expected stream helper header for messages stream")
	}
	if req.Header.Get("x-stainless-timeout") == "" {
		t.Fatalf("expected full stainless headers for messages endpoint")
	}
	if req.Header.Get("accept-language") != "*" {
		t.Fatalf("messages should include accept-language=*")
	}
	if req.Header.Get("sec-fetch-mode") != "cors" {
		t.Fatalf("messages should include sec-fetch-mode=cors")
	}
	if req.Header.Get("connection") != "keep-alive" {
		t.Fatalf("messages should include connection=keep-alive")
	}
}

func TestApplyClaudeCodeMimicHeaders_CountTokensNoStreamHelper(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/messages/count_tokens", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	applyClaudeCodeMimicHeaders(req, true, claude.EndpointCountTokens)

	if req.Header.Get("x-stainless-helper-method") != "" {
		t.Fatalf("count_tokens must not include stream helper header")
	}
	if req.Header.Get("x-stainless-timeout") != "" {
		t.Fatalf("count_tokens should use minimal stainless policy (no timeout header)")
	}
	if req.Header.Get("x-stainless-lang") == "" {
		t.Fatalf("count_tokens should still include minimal stainless headers")
	}
}

func TestApplyClaudeCodeMimicHeaders_MCPServersNoStainless(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/v1/mcp_servers", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	applyClaudeCodeMimicHeaders(req, false, claude.EndpointMCPServers)

	if req.Header.Get("user-agent") != "axios/1.13.6" {
		t.Fatalf("unexpected mcp user-agent: %q", req.Header.Get("user-agent"))
	}
	if req.Header.Get("x-stainless-lang") != "" || req.Header.Get("x-app") != "" {
		t.Fatalf("mcp_servers should not include stainless family headers")
	}
	if req.Header.Get("connection") != "close" {
		t.Fatalf("mcp_servers should include connection=close")
	}
}

func TestPassthroughClientRequestID(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/messages", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	clientHeaders := make(http.Header)
	clientHeaders.Set("x-client-request-id", "rid-123")

	passthroughClientRequestID(req, clientHeaders, claude.EndpointMessages)

	if got := req.Header.Get("x-client-request-id"); got != "rid-123" {
		t.Fatalf("x-client-request-id mismatch: got %q want %q", got, "rid-123")
	}
}

func TestPassthroughClientRequestID_GenerateWhenMissing(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/messages", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	passthroughClientRequestID(req, http.Header{}, claude.EndpointMessages)

	if got := strings.TrimSpace(req.Header.Get("x-client-request-id")); got == "" {
		t.Fatalf("x-client-request-id should be generated for messages when missing")
	}
}

func TestPassthroughClientRequestID_NoGenerateForMCP(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/v1/mcp_servers", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	passthroughClientRequestID(req, http.Header{}, claude.EndpointMCPServers)

	if got := strings.TrimSpace(req.Header.Get("x-client-request-id")); got != "" {
		t.Fatalf("mcp_servers should not auto-generate x-client-request-id, got %q", got)
	}
}

func TestBuildMCPServersRequest_UsesOfficialHostAndAuthHeader(t *testing.T) {
	s := &GatewayService{}

	req, err := s.buildMCPServersRequest(context.Background(), &Account{}, "oauth-token", "oauth")
	if err != nil {
		t.Fatalf("build oauth request: %v", err)
	}
	if req.URL.String() != claudeAPIMCPServersURL {
		t.Fatalf("mcp_servers target mismatch: got %q want %q", req.URL.String(), claudeAPIMCPServersURL)
	}
	if req.Header.Get("authorization") != "Bearer oauth-token" {
		t.Fatalf("expected oauth bearer auth header")
	}
	if req.Header.Get("x-api-key") != "" {
		t.Fatalf("oauth request should not set x-api-key")
	}

	req, err = s.buildMCPServersRequest(context.Background(), &Account{}, "api-key-token", "api_key")
	if err != nil {
		t.Fatalf("build api_key request: %v", err)
	}
	if req.Header.Get("x-api-key") != "api-key-token" {
		t.Fatalf("expected x-api-key header for api_key token type")
	}
	if req.Header.Get("authorization") != "" {
		t.Fatalf("api_key request should not set authorization")
	}
}

func TestResolveMCPPrefetchTTLs(t *testing.T) {
	if got := resolveMCPPrefetchSessionTTL(nil); got != defaultMCPPrefetchSessionTTL {
		t.Fatalf("nil config session ttl mismatch: got %v want %v", got, defaultMCPPrefetchSessionTTL)
	}
	if got := resolveMCPPrefetchCleanupTTL(nil); got != defaultMCPPrefetchCleanupTTL {
		t.Fatalf("nil config cleanup ttl mismatch: got %v want %v", got, defaultMCPPrefetchCleanupTTL)
	}

	cfg := &config.Config{}
	cfg.Gateway.MCPPrefetchSessionTTLMinutes = 12
	cfg.Gateway.MCPPrefetchCleanupIntervalMinutes = 3

	if got := resolveMCPPrefetchSessionTTL(cfg); got != 12*time.Minute {
		t.Fatalf("session ttl mismatch: got %v want %v", got, 12*time.Minute)
	}
	if got := resolveMCPPrefetchCleanupTTL(cfg); got != 3*time.Minute {
		t.Fatalf("cleanup ttl mismatch: got %v want %v", got, 3*time.Minute)
	}
}

func TestMaybeTriggerMCPServers_MessageAsyncTriggersOncePerSession(t *testing.T) {
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			Header:     make(http.Header),
		},
	}
	svc := &GatewayService{
		httpUpstream:    upstream,
		mcpTriggerCache: gocache.New(10*time.Minute, time.Minute),
	}
	account := &Account{
		ID:          9001,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
	}
	outboundCtx := &AccountOutboundContext{TokenGeneration: 1, SessionID: "session-a"}

	// Startup prefetch should be deduped by session cache.
	if err := svc.maybeTriggerMCPServers(context.Background(), nil, account, "tok", "oauth", outboundCtx, MCPTriggerStartupPrefetch); err != nil {
		t.Fatalf("startup prefetch #1 failed: %v", err)
	}
	if err := svc.maybeTriggerMCPServers(context.Background(), nil, account, "tok", "oauth", outboundCtx, MCPTriggerStartupPrefetch); err != nil {
		t.Fatalf("startup prefetch #2 failed: %v", err)
	}
	if upstream.callCount != 1 {
		t.Fatalf("startup prefetch should be deduped, got callCount=%d want=1", upstream.callCount)
	}

	// Message async should be deduped within one activated session.
	if err := svc.maybeTriggerMCPServers(context.Background(), nil, account, "tok", "oauth", outboundCtx, MCPTriggerMessageAsync); err != nil {
		t.Fatalf("message async #1 failed: %v", err)
	}
	if err := svc.maybeTriggerMCPServers(context.Background(), nil, account, "tok", "oauth", outboundCtx, MCPTriggerMessageAsync); err != nil {
		t.Fatalf("message async #2 failed: %v", err)
	}
	if upstream.callCount != 1 {
		t.Fatalf("message async should be deduped in same session, got callCount=%d want=1", upstream.callCount)
	}

	// A new session activation should trigger once again.
	nextSession := &AccountOutboundContext{TokenGeneration: 2, SessionID: "session-b"}
	if err := svc.maybeTriggerMCPServers(context.Background(), nil, account, "tok", "oauth", nextSession, MCPTriggerMessageAsync); err != nil {
		t.Fatalf("message async new session failed: %v", err)
	}
	if upstream.callCount != 2 {
		t.Fatalf("new session should trigger once, got callCount=%d want=2", upstream.callCount)
	}
}
