package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
)

const (
	claudeBillingHeaderPrefix = "x-anthropic-billing-header:"
	claudeFingerprintSalt     = "59cf53e54c78"

	cchModeOff         = "off"
	cchModePlaceholder = "placeholder"
	cchModeAttested    = "attested"
)

func (s *GatewayService) shouldInjectClaudeBillingHeader(ctx context.Context) bool {
	if s != nil && s.settingService != nil {
		return s.settingService.IsClaudeBillingHeaderEnabled(ctx)
	}
	if s == nil || s.cfg == nil {
		return true
	}
	return s.cfg.Gateway.ClaudeBillingHeaderEnabled
}

func (s *GatewayService) billingEntrypoint(ctx context.Context) string {
	if s != nil && s.settingService != nil {
		return s.settingService.GetClaudeBillingHeaderEntrypoint(ctx)
	}
	if s == nil || s.cfg == nil {
		return "cli"
	}
	v := strings.TrimSpace(s.cfg.Gateway.ClaudeBillingHeaderEntrypoint)
	if v == "" {
		return "cli"
	}
	return v
}

func (s *GatewayService) billingCCHMode(ctx context.Context) string {
	if s != nil && s.settingService != nil {
		return s.settingService.GetClaudeBillingHeaderCCHMode(ctx)
	}
	if s == nil || s.cfg == nil {
		return cchModeOff
	}
	mode := strings.ToLower(strings.TrimSpace(s.cfg.Gateway.ClaudeBillingHeaderCCHMode))
	if mode == "" {
		return cchModeOff
	}
	return mode
}

func (s *GatewayService) billingStrictMode(ctx context.Context) bool {
	if s != nil && s.settingService != nil {
		return s.settingService.IsClaudeBillingHeaderStrict(ctx)
	}
	if s == nil || s.cfg == nil {
		return false
	}
	return s.cfg.Gateway.ClaudeBillingHeaderStrict
}

func buildClaudeBillingFingerprint(messageText, version string) string {
	var b strings.Builder
	b.Grow(len(claudeFingerprintSalt) + 3 + len(version))
	b.WriteString(claudeFingerprintSalt)
	for _, idx := range [...]int{4, 7, 20} {
		if idx >= 0 && idx < len(messageText) {
			b.WriteByte(messageText[idx])
		} else {
			b.WriteByte('0')
		}
	}
	b.WriteString(version)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:3]
}

func extractFirstUserMessageText(body []byte) string {
	msgs := gjson.GetBytes(body, "messages")
	if !msgs.Exists() || !msgs.IsArray() {
		return ""
	}
	text := ""
	msgs.ForEach(func(_, msg gjson.Result) bool {
		if msg.Get("role").String() != "user" {
			return true
		}
		content := msg.Get("content")
		if !content.Exists() {
			return false
		}
		if content.Type == gjson.String {
			text = content.String()
			return false
		}
		if content.IsArray() {
			content.ForEach(func(_, block gjson.Result) bool {
				if block.Get("type").String() == "text" {
					t := block.Get("text")
					if t.Exists() && t.Type == gjson.String {
						text = t.String()
						return false
					}
				}
				return true
			})
		}
		return false
	})
	return text
}

func (s *GatewayService) buildClaudeBillingHeader(ctx context.Context, body []byte) (header string, mode string, fallback bool, err error) {
	fingerprint := buildClaudeBillingFingerprint(extractFirstUserMessageText(body), claude.ClaudeCLIVersion)
	versionWithFingerprint := claude.ClaudeCLIVersion + "." + fingerprint
	entrypoint := s.billingEntrypoint(ctx)
	mode = s.billingCCHMode(ctx)
	cch := ""
	switch mode {
	case cchModeOff:
		// no-op
	case cchModePlaceholder:
		cch = " cch=00000;"
	case cchModeAttested:
		// NOTE: native attestation algorithm lives in Bun runtime and is not
		// available in this Go service. We degrade unless strict mode is enabled.
		err = fmt.Errorf("cch mode %q not implemented in sub2api runtime", cchModeAttested)
		mode = cchModePlaceholder
		cch = " cch=00000;"
		fallback = true
	default:
		err = fmt.Errorf("unknown cch mode: %q", mode)
		mode = cchModeOff
		fallback = true
	}
	header = fmt.Sprintf("%s cc_version=%s; cc_entrypoint=%s;%s", claudeBillingHeaderPrefix, versionWithFingerprint, entrypoint, cch)
	return header, mode, fallback, err
}

func auditBillingHeader(header, mode string, fallback bool, err error) {
	if header == "" {
		return
	}
	sum := sha256.Sum256([]byte(header))
	shortHash := hex.EncodeToString(sum[:])[:12]
	if err != nil {
		logger.LegacyPrintf("service.gateway", "[billing_header] mode=%s len=%d hash=%s fallback=%v err=%v", mode, len(header), shortHash, fallback, err)
		return
	}
	logger.LegacyPrintf("service.gateway", "[billing_header] mode=%s len=%d hash=%s fallback=%v", mode, len(header), shortHash, fallback)
}

func removeBillingHeaderPrefixBlocks(system []any) []any {
	filtered := make([]any, 0, len(system))
	for _, item := range system {
		m, ok := item.(map[string]any)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		if m["type"] == "text" {
			if text, ok := m["text"].(string); ok && strings.HasPrefix(text, claudeBillingHeaderPrefix) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func upsertSystemBillingHeaderBlock(body []byte, header string) []byte {
	if len(body) == 0 || strings.TrimSpace(header) == "" {
		return body
	}

	headerBlock := map[string]any{
		"type": "text",
		"text": header,
	}

	sys := gjson.GetBytes(body, "system")
	if !sys.Exists() || sys.Type == gjson.Null {
		raw, err := json.Marshal([]any{headerBlock})
		if err != nil {
			return body
		}
		if out, ok := setJSONRawBytes(body, "system", raw); ok {
			return out
		}
		return body
	}

	if sys.Type == gjson.String {
		v := sys.String()
		var raw []byte
		var err error
		if strings.HasPrefix(v, claudeBillingHeaderPrefix) || strings.TrimSpace(v) == "" {
			raw, err = json.Marshal([]any{headerBlock})
		} else {
			raw, err = json.Marshal([]any{
				headerBlock,
				map[string]any{
					"type": "text",
					"text": v,
				},
			})
		}
		if err != nil {
			return body
		}
		if out, ok := setJSONRawBytes(body, "system", raw); ok {
			return out
		}
		return body
	}

	if sys.IsArray() {
		var parsed []any
		if err := json.Unmarshal([]byte(sys.Raw), &parsed); err != nil {
			return body
		}
		filtered := removeBillingHeaderPrefixBlocks(parsed)
		next := make([]any, 0, len(filtered)+1)
		next = append(next, headerBlock)
		next = append(next, filtered...)
		raw, err := json.Marshal(next)
		if err != nil {
			return body
		}
		if out, ok := setJSONRawBytes(body, "system", raw); ok {
			return out
		}
	}
	return body
}

func (s *GatewayService) injectClaudeOAuthBillingHeader(ctx context.Context, body []byte) ([]byte, error) {
	if !s.shouldInjectClaudeBillingHeader(ctx) {
		return body, nil
	}
	header, mode, fallback, buildErr := s.buildClaudeBillingHeader(ctx, body)
	if buildErr != nil && s.billingStrictMode(ctx) {
		return body, buildErr
	}
	auditBillingHeader(header, mode, fallback, buildErr)
	return upsertSystemBillingHeaderBlock(body, header), nil
}
