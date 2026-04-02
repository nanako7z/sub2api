package tlsfingerprint

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func parseHeaderKeys(raw string) []string {
	lines := strings.Split(raw, "\r\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, parts[0])
	}
	return out
}

func indexOf(keys []string, target string) int {
	for i, k := range keys {
		if k == target {
			return i
		}
	}
	return -1
}

func TestWriteOrderedHeaders_AnthropicMessages_SessionIDWireCaseAndOrder(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages?beta=true", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("connection", "keep-alive")
	req.Header.Set("accept", "application/json")
	req.Header.Set("x-stainless-retry-count", "0")
	req.Header.Set("x-stainless-timeout", "600")
	req.Header.Set("x-stainless-lang", "js")
	req.Header.Set("x-stainless-package-version", "0.74.0")
	req.Header.Set("x-stainless-os", "MacOS")
	req.Header.Set("x-stainless-arch", "arm64")
	req.Header.Set("x-stainless-runtime", "node")
	req.Header.Set("x-stainless-runtime-version", "v24.3.0")
	req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("authorization", "Bearer test")
	req.Header.Set("x-app", "cli")
	req.Header.Set("user-agent", "claude-cli/2.1.87 (external, cli)")
	req.Header.Set("x-claude-code-session-id", "sess-123")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("x-client-request-id", "rid-123")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("accept-encoding", "br, gzip, deflate")
	req.Header.Set("x-stainless-helper-method", "stream")

	var buf bytes.Buffer
	if err := writeOrderedHeaders(&buf, req, "api.anthropic.com"); err != nil {
		t.Fatalf("writeOrderedHeaders: %v", err)
	}

	keys := parseHeaderKeys(buf.String())
	iUA := indexOf(keys, "User-Agent")
	iSession := indexOf(keys, "X-Claude-Code-Session-Id")
	iCT := indexOf(keys, "content-type")
	iCL := indexOf(keys, "content-length")
	iHelper := indexOf(keys, "x-stainless-helper-method")
	if iUA < 0 || iSession < 0 || iCT < 0 || iCL < 0 || iHelper < 0 {
		t.Fatalf("required headers not found, keys=%v", keys)
	}
	if !(iUA < iSession && iSession < iCT) {
		t.Fatalf("unexpected order: User-Agent=%d X-Claude-Code-Session-Id=%d content-type=%d keys=%v", iUA, iSession, iCT, keys)
	}
	if !(iCL < iHelper) {
		t.Fatalf("unexpected order: content-length=%d x-stainless-helper-method=%d keys=%v", iCL, iHelper, keys)
	}
}

func TestWriteOrderedHeaders_AnthropicCountTokens_SessionIDWireCaseAndOrder(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages/count_tokens?beta=true", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("connection", "keep-alive")
	req.Header.Set("accept", "application/json")
	req.Header.Set("x-stainless-retry-count", "0")
	req.Header.Set("x-stainless-lang", "js")
	req.Header.Set("x-stainless-package-version", "0.74.0")
	req.Header.Set("x-stainless-os", "MacOS")
	req.Header.Set("x-stainless-arch", "arm64")
	req.Header.Set("x-stainless-runtime", "node")
	req.Header.Set("x-stainless-runtime-version", "v24.3.0")
	req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("authorization", "Bearer test")
	req.Header.Set("x-app", "cli")
	req.Header.Set("user-agent", "claude-cli/2.1.87 (external, cli)")
	req.Header.Set("x-claude-code-session-id", "sess-ct-123")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("x-client-request-id", "rid-ct-123")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("accept-encoding", "br, gzip, deflate")

	var buf bytes.Buffer
	if err := writeOrderedHeaders(&buf, req, "api.anthropic.com"); err != nil {
		t.Fatalf("writeOrderedHeaders: %v", err)
	}

	keys := parseHeaderKeys(buf.String())
	iUA := indexOf(keys, "User-Agent")
	iSession := indexOf(keys, "X-Claude-Code-Session-Id")
	iCT := indexOf(keys, "content-type")
	if iUA < 0 || iSession < 0 || iCT < 0 {
		t.Fatalf("required headers not found, keys=%v", keys)
	}
	if !(iUA < iSession && iSession < iCT) {
		t.Fatalf("unexpected order: User-Agent=%d X-Claude-Code-Session-Id=%d content-type=%d keys=%v", iUA, iSession, iCT, keys)
	}
}
