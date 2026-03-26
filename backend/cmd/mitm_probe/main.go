package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

func applyMimicHeaders(req *http.Request, endpointID string, isStream bool) {
	profile := claude.HeaderProfileForEndpoint(endpointID)
	req.Header.Set("user-agent", profile.UserAgent)
	req.Header.Set("accept", profile.Accept)
	req.Header.Set("accept-encoding", profile.AcceptEncoding)
	req.Header.Set("anthropic-beta", profile.BetaHeader)
	if profile.AcceptLanguage != "" {
		req.Header.Set("accept-language", profile.AcceptLanguage)
	}
	if profile.SecFetchMode != "" {
		req.Header.Set("sec-fetch-mode", profile.SecFetchMode)
	}
	if profile.Connection != "" {
		req.Header.Set("connection", profile.Connection)
	}

	switch profile.StainlessPolicy {
	case "minimal":
		req.Header.Set("x-stainless-lang", claude.DefaultHeaders["X-Stainless-Lang"])
		req.Header.Set("x-stainless-package-version", claude.DefaultHeaders["X-Stainless-Package-Version"])
		req.Header.Set("x-stainless-os", claude.DefaultHeaders["X-Stainless-OS"])
		req.Header.Set("x-stainless-arch", claude.DefaultHeaders["X-Stainless-Arch"])
		req.Header.Set("x-stainless-runtime", claude.DefaultHeaders["X-Stainless-Runtime"])
		req.Header.Set("x-stainless-runtime-version", claude.DefaultHeaders["X-Stainless-Runtime-Version"])
		req.Header.Set("x-stainless-retry-count", claude.DefaultHeaders["X-Stainless-Retry-Count"])
		req.Header.Set("x-app", claude.DefaultHeaders["X-App"])
		req.Header.Set("anthropic-dangerous-direct-browser-access", claude.DefaultHeaders["Anthropic-Dangerous-Direct-Browser-Access"])
	case "full":
		req.Header.Set("x-stainless-lang", claude.DefaultHeaders["X-Stainless-Lang"])
		req.Header.Set("x-stainless-package-version", claude.DefaultHeaders["X-Stainless-Package-Version"])
		req.Header.Set("x-stainless-os", claude.DefaultHeaders["X-Stainless-OS"])
		req.Header.Set("x-stainless-arch", claude.DefaultHeaders["X-Stainless-Arch"])
		req.Header.Set("x-stainless-runtime", claude.DefaultHeaders["X-Stainless-Runtime"])
		req.Header.Set("x-stainless-runtime-version", claude.DefaultHeaders["X-Stainless-Runtime-Version"])
		req.Header.Set("x-stainless-retry-count", claude.DefaultHeaders["X-Stainless-Retry-Count"])
		req.Header.Set("x-stainless-timeout", claude.DefaultHeaders["X-Stainless-Timeout"])
		req.Header.Set("x-app", claude.DefaultHeaders["X-App"])
		req.Header.Set("anthropic-dangerous-direct-browser-access", claude.DefaultHeaders["Anthropic-Dangerous-Direct-Browser-Access"])
	}

	if isStream && profile.StreamHeaderPolicy == "helper_stream" {
		req.Header.Set("x-stainless-helper-method", "stream")
	}
}

func sendReq(
	ctx context.Context,
	upstream service.HTTPUpstream,
	account *service.Account,
	proxyURL string,
	req *http.Request,
	enableTLSFingerprint bool,
) {
	resp, err := upstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, enableTLSFingerprint)
	if err != nil {
		fmt.Printf("[probe] %s %s -> request error: %v\n", req.Method, req.URL.String(), err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	fmt.Printf("[probe] %s %s -> status=%d\n", req.Method, req.URL.String(), resp.StatusCode)
}

func resolveClientRequestID(provided string) string {
	if v := strings.TrimSpace(provided); v != "" {
		return v
	}
	return uuid.NewString()
}

func main() {
	var (
		proxyURL        = flag.String("proxy", "http://127.0.0.1:8083", "HTTP proxy URL used by MITM")
		repeat          = flag.Int("repeat", 2, "repeat rounds")
		tlsFP           = flag.Bool("tls-fp", true, "enable sub2api TLS fingerprint path")
		clientRequestID = flag.String("client-request-id", "", "optional x-client-request-id passthrough value")
	)
	flag.Parse()
	if *repeat <= 0 {
		*repeat = 1
	}

	upstream := repository.NewHTTPUpstream(nil)
	account := &service.Account{
		ID:          9001,
		Name:        "mitm-probe-oauth",
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeOAuth,
		Concurrency: 1,
	}

	messageBody := []byte(`{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"user","content":"mitm probe messages"}]}`)
	countBody := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"mitm probe count tokens"}]}`)

	for i := 0; i < *repeat; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)

		msgReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages?beta=true", bytes.NewReader(messageBody))
		if err == nil {
			applyMimicHeaders(msgReq, claude.EndpointMessages, false)
			msgReq.Header.Set("content-type", "application/json")
			msgReq.Header.Set("anthropic-version", "2023-06-01")
			msgReq.Header.Set("authorization", "Bearer invalid-oauth-token")
			msgReq.Header.Set("anthropic-beta", claude.DefaultBetaHeader)
			msgReq.Header.Set("x-client-request-id", resolveClientRequestID(*clientRequestID))
			sendReq(ctx, upstream, account, *proxyURL, msgReq, *tlsFP)
		}

		ctReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages/count_tokens?beta=true", bytes.NewReader(countBody))
		if err == nil {
			applyMimicHeaders(ctReq, claude.EndpointCountTokens, false)
			ctReq.Header.Set("content-type", "application/json")
			ctReq.Header.Set("anthropic-version", "2023-06-01")
			ctReq.Header.Set("authorization", "Bearer invalid-oauth-token")
			ctReq.Header.Set("x-client-request-id", resolveClientRequestID(*clientRequestID))
			sendReq(ctx, upstream, account, *proxyURL, ctReq, *tlsFP)
		}

		mcpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/mcp_servers?limit=1000", nil)
		if err == nil {
			applyMimicHeaders(mcpReq, claude.EndpointMCPServers, false)
			mcpReq.Header.Set("content-type", "application/json")
			mcpReq.Header.Set("anthropic-version", "2023-06-01")
			mcpReq.Header.Set("authorization", "Bearer invalid-oauth-token")
			sendReq(ctx, upstream, account, *proxyURL, mcpReq, *tlsFP)
		}

		usageFetcher := repository.NewClaudeUsageFetcher(upstream)
		_, usageErr := usageFetcher.FetchUsageWithOptions(ctx, &service.ClaudeUsageFetchOptions{
			AccessToken:          "invalid-oauth-token",
			ProxyURL:             *proxyURL,
			EnableTLSFingerprint: *tlsFP,
			AccountID:            account.ID,
		})
		if usageErr != nil {
			fmt.Printf("[probe] GET /api/oauth/usage -> expected error: %v\n", usageErr)
		}

		cancel()
	}
}
