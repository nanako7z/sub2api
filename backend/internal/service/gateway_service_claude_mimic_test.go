package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
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

	if req.Header.Get("user-agent") != claude.HeaderProfileForEndpoint(claude.EndpointMCPServers).UserAgent {
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
	got := strings.TrimSpace(req.Header.Get("x-client-request-id"))
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(got) {
		t.Fatalf("x-client-request-id should be UUIDv4, got %q", got)
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

	req, err := s.buildMCPServersRequest(context.Background(), &Account{}, "oauth-token", "oauth", nil)
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

	req, err = s.buildMCPServersRequest(context.Background(), &Account{}, "api-key-token", "api_key", nil)
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

func TestBuildUpstreamRequest_OAuthMimic_PassthroughThenOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(nil))
	c.Request.Header.Set("x-test-trace", "trace-123")
	c.Request.Header.Set("x-api-key", "must-not-forward")
	c.Request.Header.Set("user-agent", "custom-client/1.0")
	c.Request.Header.Set("x-stainless-os", "Windows")
	c.Request.Header.Set("x-stainless-arch", "x86")
	c.Request.Header.Set("anthropic-beta", "downstream-extra-beta")

	account := &Account{
		ID:          1001,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
	}
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16}`)

	svc := &GatewayService{}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-5", false, true)
	if err != nil {
		t.Fatalf("buildUpstreamRequest failed: %v", err)
	}

	if got := req.Header.Get("x-test-trace"); got != "trace-123" {
		t.Fatalf("passthrough header mismatch: got %q want %q", got, "trace-123")
	}
	if got := req.Header.Get("x-api-key"); got != "" {
		t.Fatalf("conflicting auth header should be stripped, got %q", got)
	}
	if got := req.Header.Get("user-agent"); got != claude.HeaderProfileForEndpoint(claude.EndpointMessages).UserAgent {
		t.Fatalf("user-agent must be overridden by mimic profile, got %q", got)
	}
	if got := req.Header.Get("x-stainless-os"); got != claude.DefaultHeaders["X-Stainless-OS"] {
		t.Fatalf("x-stainless-os must be overridden, got %q want %q", got, claude.DefaultHeaders["X-Stainless-OS"])
	}
	if got := req.Header.Get("x-stainless-arch"); got != claude.DefaultHeaders["X-Stainless-Arch"] {
		t.Fatalf("x-stainless-arch must be overridden, got %q want %q", got, claude.DefaultHeaders["X-Stainless-Arch"])
	}
	if beta := req.Header.Get("anthropic-beta"); !strings.Contains(beta, "downstream-extra-beta") || !strings.Contains(beta, claude.BetaOAuth) {
		t.Fatalf("anthropic-beta should merge required + downstream tokens, got %q", beta)
	}
}

func TestBuildCountTokensRequest_OAuthMimic_PassthroughThenOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(nil))
	c.Request.Header.Set("x-test-meta", "meta-1")
	c.Request.Header.Set("authorization", "must-not-forward")
	c.Request.Header.Set("user-agent", "custom-client/2.0")
	c.Request.Header.Set("x-stainless-runtime-version", "v0.0.1")
	c.Request.Header.Set("anthropic-beta", "ct-extra-beta")

	account := &Account{
		ID:          1002,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
	}
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)

	svc := &GatewayService{}
	req, err := svc.buildCountTokensRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-5", true, true)
	if err != nil {
		t.Fatalf("buildCountTokensRequest failed: %v", err)
	}

	if got := req.Header.Get("x-test-meta"); got != "meta-1" {
		t.Fatalf("passthrough header mismatch: got %q want %q", got, "meta-1")
	}
	if got := req.Header.Get("authorization"); got != "Bearer oauth-token" {
		t.Fatalf("authorization should use upstream oauth token, got %q", got)
	}
	if got := req.Header.Get("user-agent"); got != claude.HeaderProfileForEndpoint(claude.EndpointCountTokens).UserAgent {
		t.Fatalf("user-agent must be overridden by mimic profile, got %q", got)
	}
	if got := req.Header.Get("x-stainless-runtime-version"); got != claude.DefaultHeaders["X-Stainless-Runtime-Version"] {
		t.Fatalf("x-stainless-runtime-version must be overridden, got %q want %q", got, claude.DefaultHeaders["X-Stainless-Runtime-Version"])
	}
	if beta := req.Header.Get("anthropic-beta"); !strings.Contains(beta, "ct-extra-beta") || !strings.Contains(beta, claude.BetaOAuth) {
		t.Fatalf("anthropic-beta should merge required + downstream tokens, got %q", beta)
	}
}

func TestBuildUpstreamRequest_OAuthMimic_FingerprintUnificationDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(nil))
	c.Request.Header.Set("user-agent", "custom-client/1.0")
	c.Request.Header.Set("x-stainless-os", "Windows")
	c.Request.Header.Set("x-stainless-arch", "x86")
	c.Request.Header.Set("anthropic-beta", "downstream-extra-beta")

	account := &Account{
		ID:          1010,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
	}
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16}`)

	settings := NewSettingService(&gatewayToggleSettingRepoStub{
		values: map[string]string{
			SettingKeyEnableFingerprintUnification: "false",
		},
	}, &config.Config{})

	svc := &GatewayService{settingService: settings}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-5", false, true)
	if err != nil {
		t.Fatalf("buildUpstreamRequest failed: %v", err)
	}

	if got := req.Header.Get("user-agent"); got != "custom-client/1.0" {
		t.Fatalf("user-agent should keep downstream value when fingerprint unification disabled, got %q", got)
	}
	if got := req.Header.Get("x-stainless-os"); got != "Windows" {
		t.Fatalf("x-stainless-os should keep downstream value when fingerprint unification disabled, got %q", got)
	}
	if got := req.Header.Get("x-stainless-arch"); got != "x86" {
		t.Fatalf("x-stainless-arch should keep downstream value when fingerprint unification disabled, got %q", got)
	}
}

func TestBuildCountTokensRequest_OAuthMimic_FingerprintUnificationDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(nil))
	c.Request.Header.Set("user-agent", "custom-client/2.0")
	c.Request.Header.Set("x-stainless-runtime-version", "v0.0.1")
	c.Request.Header.Set("anthropic-beta", "ct-extra-beta")

	account := &Account{
		ID:          1011,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
	}
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)

	settings := NewSettingService(&gatewayToggleSettingRepoStub{
		values: map[string]string{
			SettingKeyEnableFingerprintUnification: "false",
		},
	}, &config.Config{})

	svc := &GatewayService{settingService: settings}
	req, err := svc.buildCountTokensRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-5", true, true)
	if err != nil {
		t.Fatalf("buildCountTokensRequest failed: %v", err)
	}

	if got := req.Header.Get("user-agent"); got != "custom-client/2.0" {
		t.Fatalf("user-agent should keep downstream value when fingerprint unification disabled, got %q", got)
	}
	if got := req.Header.Get("x-stainless-runtime-version"); got != "v0.0.1" {
		t.Fatalf("x-stainless-runtime-version should keep downstream value when fingerprint unification disabled, got %q", got)
	}
}

func TestGatewayToggleDefaults(t *testing.T) {
	svc := &GatewayService{}
	if !svc.fingerprintUnificationEnabled(context.Background()) {
		t.Fatalf("fingerprint unification should default to true")
	}
	if svc.metadataPassthroughEnabled(context.Background()) {
		t.Fatalf("metadata passthrough should default to false")
	}
}

func TestGatewayToggle_MetadataPassthroughSetting(t *testing.T) {
	svcFalse := &GatewayService{
		settingService: NewSettingService(&gatewayToggleSettingRepoStub{
			values: map[string]string{SettingKeyEnableMetadataPassthrough: "false"},
		}, &config.Config{}),
	}
	if svcFalse.metadataPassthroughEnabled(context.Background()) {
		t.Fatalf("metadata passthrough should be false when setting=false")
	}

	svcTrue := &GatewayService{
		settingService: NewSettingService(&gatewayToggleSettingRepoStub{
			values: map[string]string{SettingKeyEnableMetadataPassthrough: "true"},
		}, &config.Config{}),
	}
	if !svcTrue.metadataPassthroughEnabled(context.Background()) {
		t.Fatalf("metadata passthrough should be true when setting=true")
	}
}

type gatewayToggleSettingRepoStub struct {
	values map[string]string
}

func (s *gatewayToggleSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (s *gatewayToggleSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}
func (s *gatewayToggleSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (s *gatewayToggleSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := s.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}
func (s *gatewayToggleSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (s *gatewayToggleSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (s *gatewayToggleSettingRepoStub) Delete(context.Context, string) error { return nil }

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

func TestMaybeTriggerMCPServers_ScopeGatedOneShotByTokenGeneration(t *testing.T) {
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
		},
	}
	svc := &GatewayService{
		httpUpstream:    upstream,
		mcpTriggerCache: gocache.New(10*time.Minute, time.Minute),
	}
	accountWithoutScope := &Account{
		ID:          9001,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"scope": "user:profile user:inference",
		},
	}
	account := &Account{
		ID:          9002,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"scope": "user:profile user:inference user:mcp_servers",
		},
	}
	outboundCtx := &AccountOutboundContext{TokenGeneration: 1, SessionID: "session-a"}

	// Missing user:mcp_servers scope: should not trigger.
	if err := svc.maybeTriggerMCPServers(context.Background(), nil, accountWithoutScope, "tok", "oauth", outboundCtx, MCPTriggerStartupPrefetch); err != nil {
		t.Fatalf("missing-scope trigger failed: %v", err)
	}
	if upstream.callCount != 0 {
		t.Fatalf("missing scope should not call upstream, got callCount=%d", upstream.callCount)
	}

	// Non-startup reason should not trigger.
	if err := svc.maybeTriggerMCPServers(context.Background(), nil, account, "tok", "oauth", outboundCtx, MCPTriggerMessageAsync); err != nil {
		t.Fatalf("message_async trigger failed: %v", err)
	}
	if upstream.callCount != 0 {
		t.Fatalf("non-startup reason should not call upstream, got callCount=%d", upstream.callCount)
	}

	// Startup prefetch: one-shot for this token generation.
	if err := svc.maybeTriggerMCPServers(context.Background(), nil, account, "tok", "oauth", outboundCtx, MCPTriggerStartupPrefetch); err != nil {
		t.Fatalf("startup prefetch #1 failed: %v", err)
	}
	if err := svc.maybeTriggerMCPServers(context.Background(), nil, account, "tok", "oauth", outboundCtx, MCPTriggerStartupPrefetch); err != nil {
		t.Fatalf("startup prefetch #2 failed: %v", err)
	}
	if upstream.callCount != 1 {
		t.Fatalf("same token generation should dedupe, got callCount=%d want=1", upstream.callCount)
	}

	// Token generation changed: allow one more trigger.
	outboundCtx.TokenGeneration = 2
	if err := svc.maybeTriggerMCPServers(context.Background(), nil, account, "tok", "oauth", outboundCtx, MCPTriggerStartupPrefetch); err != nil {
		t.Fatalf("startup prefetch for new generation failed: %v", err)
	}
	if upstream.callCount != 2 {
		t.Fatalf("new token generation should trigger again, got callCount=%d want=2", upstream.callCount)
	}
}

func TestTriggerMCPServersAsync_EligibleOneShot(t *testing.T) {
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
		},
	}
	svc := &GatewayService{
		httpUpstream:    upstream,
		mcpTriggerCache: gocache.New(10*time.Minute, time.Minute),
	}
	account := &Account{
		ID:          9002,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"scope": "user:profile user:inference user:mcp_servers",
		},
	}
	outboundCtx := &AccountOutboundContext{TokenGeneration: 7, SessionID: "session-async"}

	svc.triggerMCPServersAsync(account, "tok", "oauth", outboundCtx, MCPTriggerStartupPrefetch)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if upstream.callCount >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if upstream.callCount != 1 {
		t.Fatalf("eligible async prefetch should call upstream once, got callCount=%d", upstream.callCount)
	}

	svc.triggerMCPServersAsync(account, "tok", "oauth", outboundCtx, MCPTriggerStartupPrefetch)
	time.Sleep(40 * time.Millisecond)
	if upstream.callCount != 1 {
		t.Fatalf("same token generation should stay one-shot in async path, got callCount=%d", upstream.callCount)
	}
}
