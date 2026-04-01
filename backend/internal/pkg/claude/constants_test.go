package claude

import "testing"

func TestHeaderProfileForEndpoint(t *testing.T) {
	tests := []struct {
		endpointID         string
		wantEndpointID     string
		wantStreamPolicy   string
		wantStainless      string
		wantForceOfficial  bool
		wantAcceptContains string
		wantUAContains     string
	}{
		{
			endpointID:         EndpointMessages,
			wantEndpointID:     EndpointMessages,
			wantStreamPolicy:   "helper_stream",
			wantStainless:      "full",
			wantForceOfficial:  false,
			wantAcceptContains: "application/json",
			wantUAContains:     "(external, cli)",
		},
		{
			endpointID:         EndpointCountTokens,
			wantEndpointID:     EndpointCountTokens,
			wantStreamPolicy:   "none",
			wantStainless:      "minimal",
			wantForceOfficial:  false,
			wantAcceptContains: "application/json",
			wantUAContains:     "(external, cli)",
		},
		{
			endpointID:         EndpointMCPServers,
			wantEndpointID:     EndpointMCPServers,
			wantStreamPolicy:   "none",
			wantStainless:      "none",
			wantForceOfficial:  true,
			wantAcceptContains: "text/plain",
			wantUAContains:     "axios/1.13.6",
		},
		{
			endpointID:         EndpointOAuthUsage,
			wantEndpointID:     EndpointOAuthUsage,
			wantStreamPolicy:   "none",
			wantStainless:      "none",
			wantForceOfficial:  true,
			wantAcceptContains: "text/plain",
			wantUAContains:     "claude-code/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.endpointID, func(t *testing.T) {
			p := HeaderProfileForEndpoint(tt.endpointID)
			if p.EndpointID != tt.wantEndpointID {
				t.Fatalf("endpoint id mismatch: got %q want %q", p.EndpointID, tt.wantEndpointID)
			}
			if p.StreamHeaderPolicy != tt.wantStreamPolicy {
				t.Fatalf("stream policy mismatch: got %q want %q", p.StreamHeaderPolicy, tt.wantStreamPolicy)
			}
			if p.StainlessPolicy != tt.wantStainless {
				t.Fatalf("stainless policy mismatch: got %q want %q", p.StainlessPolicy, tt.wantStainless)
			}
			if p.ForceOfficialHost != tt.wantForceOfficial {
				t.Fatalf("force official host mismatch: got %t want %t", p.ForceOfficialHost, tt.wantForceOfficial)
			}
			if tt.wantAcceptContains != "" && !contains(p.Accept, tt.wantAcceptContains) {
				t.Fatalf("accept mismatch: got %q expected to contain %q", p.Accept, tt.wantAcceptContains)
			}
			if tt.wantUAContains != "" && !contains(p.UserAgent, tt.wantUAContains) {
				t.Fatalf("user-agent mismatch: got %q expected to contain %q", p.UserAgent, tt.wantUAContains)
			}
		})
	}
}

func TestVersionedUserAgentsUseVersionConstant(t *testing.T) {
	if got := DefaultHeaders["User-Agent"]; got != ClaudeCLIUserAgent() {
		t.Fatalf("default user-agent mismatch: got %q want %q", got, ClaudeCLIUserAgent())
	}
	if got := HeaderProfileForEndpoint(EndpointMessages).UserAgent; got != ClaudeCLIUserAgent() {
		t.Fatalf("messages user-agent mismatch: got %q want %q", got, ClaudeCLIUserAgent())
	}
	if got := HeaderProfileForEndpoint(EndpointCountTokens).UserAgent; got != ClaudeCLIUserAgent() {
		t.Fatalf("count_tokens user-agent mismatch: got %q want %q", got, ClaudeCLIUserAgent())
	}
	if got := HeaderProfileForEndpoint(EndpointOAuthUsage).UserAgent; got != ClaudeCodeUserAgent() {
		t.Fatalf("oauth_usage user-agent mismatch: got %q want %q", got, ClaudeCodeUserAgent())
	}
	if got := HeaderProfileForEndpoint(EndpointMCPServers).UserAgent; contains(got, ClaudeCLIVersion) {
		t.Fatalf("mcp_servers should keep non-versioned user-agent behavior, got %q", got)
	}
}

func TestHeaderProfileForEndpoint_DefaultFallsBackToMessages(t *testing.T) {
	p := HeaderProfileForEndpoint("unknown_endpoint")
	if p.EndpointID != EndpointMessages {
		t.Fatalf("expected fallback endpoint %q, got %q", EndpointMessages, p.EndpointID)
	}
	if p.StreamHeaderPolicy != "helper_stream" {
		t.Fatalf("expected helper_stream fallback, got %q", p.StreamHeaderPolicy)
	}
	if p.StainlessPolicy != "full" {
		t.Fatalf("expected full stainless fallback, got %q", p.StainlessPolicy)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
