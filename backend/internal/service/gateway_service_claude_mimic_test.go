package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
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
