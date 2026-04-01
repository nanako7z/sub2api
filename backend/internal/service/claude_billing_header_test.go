package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
)

func TestBuildClaudeBillingFingerprint_Deterministic(t *testing.T) {
	got := buildClaudeBillingFingerprint("hello world", "2.1.87")
	const want = "b16"
	if got != want {
		t.Fatalf("fingerprint mismatch: got=%q want=%q", got, want)
	}
}

func TestExtractFirstUserMessageText(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":"ignore"},{"role":"user","content":[{"type":"text","text":"hello text"},{"type":"image","source":"x"}]}]}`)
	got := extractFirstUserMessageText(body)
	if got != "hello text" {
		t.Fatalf("first user text mismatch: got=%q", got)
	}
}

func TestUpsertSystemBillingHeaderBlock_PrependsAndReplacesExisting(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=old; cc_entrypoint=cli;"},{"type":"text","text":"keep-1"},{"type":"text","text":"keep-2"}]}`)
	out := upsertSystemBillingHeaderBlock(body, "x-anthropic-billing-header: cc_version=new; cc_entrypoint=cli;")
	sys := gjson.GetBytes(out, "system")
	if !sys.IsArray() {
		t.Fatalf("system must be array")
	}
	if sys.Array()[0].Get("text").String() != "x-anthropic-billing-header: cc_version=new; cc_entrypoint=cli;" {
		t.Fatalf("billing header must be first block")
	}
	raw := sys.Raw
	if strings.Count(raw, "x-anthropic-billing-header:") != 1 {
		t.Fatalf("billing header block should be unique, raw=%s", raw)
	}
	if !strings.Contains(raw, "keep-1") || !strings.Contains(raw, "keep-2") {
		t.Fatalf("original non-billing blocks should be preserved, raw=%s", raw)
	}
}

func TestInjectClaudeOAuthBillingHeader_StrictAttestedModeFails(t *testing.T) {
	svc := &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				ClaudeBillingHeaderEnabled:    true,
				ClaudeBillingHeaderCCHMode:    "attested",
				ClaudeBillingHeaderStrict:     true,
				ClaudeBillingHeaderEntrypoint: "cli",
			},
		},
	}
	body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	_, err := svc.injectClaudeOAuthBillingHeader(context.Background(), body)
	if err == nil {
		t.Fatalf("strict attested mode should fail when attestation implementation is unavailable")
	}
}

func TestInjectClaudeOAuthBillingHeader_SystemFirst(t *testing.T) {
	svc := &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				ClaudeBillingHeaderEnabled:    true,
				ClaudeBillingHeaderCCHMode:    "off",
				ClaudeBillingHeaderEntrypoint: "cli",
			},
		},
	}
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello world"}]}],"max_tokens":16}`)
	out, err := svc.injectClaudeOAuthBillingHeader(context.Background(), body)
	if err != nil {
		t.Fatalf("injectClaudeOAuthBillingHeader failed: %v", err)
	}
	sys := gjson.GetBytes(out, "system")
	if !sys.IsArray() || len(sys.Array()) == 0 {
		t.Fatalf("system should be non-empty array after billing injection")
	}
	first := sys.Array()[0].Get("text").String()
	if !strings.HasPrefix(first, "x-anthropic-billing-header: cc_version=") {
		t.Fatalf("system[0] should be billing header block, got=%q", first)
	}
	if strings.Contains(first, " cch=") {
		t.Fatalf("cch should be absent in off mode, got=%q", first)
	}
}
