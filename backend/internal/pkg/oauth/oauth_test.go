package oauth

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSessionStore_Stop_Idempotent(t *testing.T) {
	store := NewSessionStore()

	store.Stop()
	store.Stop()

	select {
	case <-store.stopCh:
		// ok
	case <-time.After(time.Second):
		t.Fatal("stopCh 未关闭")
	}
}

func TestSessionStore_Stop_Concurrent(t *testing.T) {
	store := NewSessionStore()

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Stop()
		}()
	}

	wg.Wait()

	select {
	case <-store.stopCh:
		// ok
	case <-time.After(time.Second):
		t.Fatal("stopCh 未关闭")
	}
}

func TestScopes_IncludeMCPServers(t *testing.T) {
	if !strings.Contains(ScopeOAuth, "user:mcp_servers") {
		t.Fatalf("ScopeOAuth must include mcp scope: %q", ScopeOAuth)
	}
	if !strings.Contains(ScopeAPI, "user:mcp_servers") {
		t.Fatalf("ScopeAPI must include mcp scope: %q", ScopeAPI)
	}
}
