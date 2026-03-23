package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type identityCacheWithFingerprintAccountTestStub struct {
	identityCacheStub
	fp *Fingerprint
}

func (s *identityCacheWithFingerprintAccountTestStub) GetFingerprint(_ context.Context, _ int64) (*Fingerprint, error) {
	return s.fp, nil
}

func TestAccountTestService_AnthropicAPIKeyTestConnection_MimicsHeadersAndTLS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newSoraTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader("data: {\"type\":\"message_start\"}\n\ndata: [DONE]\n\n"))

	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	account := &Account{
		ID:          901,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://api.anthropic.com",
		},
		Extra: map[string]any{
			"session_id_masking_enabled": true,
		},
	}
	svc := &AccountTestService{
		httpUpstream:    upstream,
		cfg:             &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		identityService: NewIdentityService(&identityCacheWithFingerprintAccountTestStub{fp: &Fingerprint{ClientID: "acct-test-client-id"}, identityCacheStub: identityCacheStub{maskedSessionID: "12121212-3434-4567-8787-909090909090"}}, nil),
	}

	err := svc.testClaudeAccountConnection(ctx, account, "claude-sonnet-4-5")
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	require.True(t, upstream.tlsFlags[0], "Anthropic API key account test should use TLS fingerprint")
	require.Equal(t, "test-api-key", req.Header.Get("x-api-key"))
	require.Equal(t, claude.DefaultHeaders["User-Agent"], req.Header.Get("User-Agent"))
	require.Equal(t, claude.DefaultHeaders["X-Stainless-Lang"], req.Header.Get("X-Stainless-Lang"))
	require.NotContains(t, req.Header.Get("anthropic-beta"), claude.BetaOAuth)
	require.Contains(t, req.Header.Get("anthropic-beta"), claude.BetaClaudeCode)

	reqBody, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	parsed := ParseMetadataUserID(gjson.GetBytes(reqBody, "metadata.user_id").String())
	require.NotNil(t, parsed)
	require.Equal(t, "acct-test-client-id", parsed.DeviceID)
	require.Equal(t, "12121212-3434-4567-8787-909090909090", parsed.SessionID)
	require.Contains(t, recorder.Body.String(), "test_complete")
}
