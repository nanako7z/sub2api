//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockCache 记录所有 SetSessionAccountID / RefreshSessionTTL 调用，用于断言。
type mockCache struct {
	mu      sync.Mutex
	setBind []mockSetCall
	refresh []mockRefreshCall
}

type mockSetCall struct {
	groupID    int64
	sessionKey string
	accountID  int64
}

type mockRefreshCall struct {
	groupID    int64
	sessionKey string
}

func (m *mockCache) SetSessionAccountID(_ context.Context, groupID int64, sessionKey string, accountID int64, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setBind = append(m.setBind, mockSetCall{groupID, sessionKey, accountID})
	return nil
}

func (m *mockCache) RefreshSessionTTL(_ context.Context, groupID int64, sessionKey string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refresh = append(m.refresh, mockRefreshCall{groupID, sessionKey})
	return nil
}

func (m *mockCache) GetSessionAccountID(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}

func (m *mockCache) DeleteSessionAccountID(_ context.Context, _ int64, _ string) error {
	return nil
}

func (m *mockCache) AddStickySessionReverse(_ context.Context, _ int64, _ int64, _ string, _ time.Duration) error {
	return nil
}
func (m *mockCache) RemoveStickySessionReverse(_ context.Context, _ int64, _ int64, _ string) error {
	return nil
}
func (m *mockCache) GetStickySessionCounts(_ context.Context, _ []int64) (map[int64]int, error) {
	return nil, nil
}

// buildSvcWithCache 构建一个带 mockCache 的最小 GatewayService。
func buildSvcWithCache(cache *mockCache) *GatewayService {
	return &GatewayService{cache: cache}
}

// ctxWithAffinity 注入 affinity hash 到 context。
func ctxWithAffinity(affinityHash string) context.Context {
	return WithAffinityHash(context.Background(), affinityHash)
}

// ============ setAffinityBinding 去重测试 ============

// TestSetAffinityBinding_SkipsWhenAffinityEqualsSessionHash 验证当
// affinityHash == sessionHash 时，setAffinityBinding 不写入额外绑定（去重守卫）。
func TestSetAffinityBinding_SkipsWhenAffinityEqualsSessionHash(t *testing.T) {
	cache := &mockCache{}
	svc := buildSvcWithCache(cache)

	sessionHash := "abc123"
	// affinityHash == sessionHash → 应跳过，不写入
	ctx := ctxWithAffinity(sessionHash)
	gid := int64(1)
	svc.setAffinityBinding(ctx, &gid, sessionHash, 42)

	cache.mu.Lock()
	defer cache.mu.Unlock()
	require.Empty(t, cache.setBind, "affinityHash == sessionHash should not trigger extra SetSessionAccountID")
}

// TestSetAffinityBinding_WritesWhenAffinityDiffersFromSessionHash 验证当
// affinityHash != sessionHash 时，setAffinityBinding 正常写入 affinity 绑定。
func TestSetAffinityBinding_WritesWhenAffinityDiffersFromSessionHash(t *testing.T) {
	cache := &mockCache{}
	svc := buildSvcWithCache(cache)

	sessionHash := "session-aaa"
	affinityHash := "affinity-bbb"
	ctx := ctxWithAffinity(affinityHash)
	gid := int64(1)
	svc.setAffinityBinding(ctx, &gid, sessionHash, 42)

	cache.mu.Lock()
	defer cache.mu.Unlock()
	require.Len(t, cache.setBind, 1, "affinity != session should write affinity binding")
	require.Equal(t, affinityHash, cache.setBind[0].sessionKey, "should write affinity hash as the key")
	require.Equal(t, int64(42), cache.setBind[0].accountID)
}

// TestSetAffinityBinding_SkipsWhenAffinityEmpty 验证 affinity hash 为空时不写入。
func TestSetAffinityBinding_SkipsWhenAffinityEmpty(t *testing.T) {
	cache := &mockCache{}
	svc := buildSvcWithCache(cache)

	ctx := ctxWithAffinity("")
	gid := int64(1)
	svc.setAffinityBinding(ctx, &gid, "session-xxx", 42)

	cache.mu.Lock()
	defer cache.mu.Unlock()
	require.Empty(t, cache.setBind, "empty affinity hash should not write any binding")
}

// TestSetAffinityBinding_SkipsWhenCacheNil 验证 cache 为 nil 时不 panic。
func TestSetAffinityBinding_SkipsWhenCacheNil(t *testing.T) {
	svc := &GatewayService{cache: nil}
	ctx := ctxWithAffinity("some-affinity")
	gid := int64(1)
	require.NotPanics(t, func() {
		svc.setAffinityBinding(ctx, &gid, "session-xxx", 42)
	})
}

// ============ bindSessionAndAffinity 测试 ============

// TestBindSessionAndAffinity_WritesBothBindings 验证同时写入 session + affinity 两个绑定。
func TestBindSessionAndAffinity_WritesBothBindings(t *testing.T) {
	cache := &mockCache{}
	svc := buildSvcWithCache(cache)

	sessionHash := "session-111"
	affinityHash := "affinity-222"
	ctx := ctxWithAffinity(affinityHash)
	gid := int64(5)

	svc.bindSessionAndAffinity(ctx, &gid, sessionHash, 99)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	require.Len(t, cache.setBind, 2, "should write session binding + affinity binding")

	keys := map[string]bool{}
	for _, c := range cache.setBind {
		keys[c.sessionKey] = true
		require.Equal(t, int64(99), c.accountID)
		require.Equal(t, int64(5), c.groupID)
	}
	require.True(t, keys[sessionHash], "session hash should be written")
	require.True(t, keys[affinityHash], "affinity hash should be written")
}

// TestBindSessionAndAffinity_DeduplicatesWhenAffinityEqualsSession 验证当
// affinityHash == sessionHash 时只写一次（不重复写同一个 key）。
func TestBindSessionAndAffinity_DeduplicatesWhenAffinityEqualsSession(t *testing.T) {
	cache := &mockCache{}
	svc := buildSvcWithCache(cache)

	hash := "same-hash"
	ctx := ctxWithAffinity(hash)
	gid := int64(1)

	svc.bindSessionAndAffinity(ctx, &gid, hash, 7)

	cache.mu.Lock()
	defer cache.mu.Unlock()
	require.Len(t, cache.setBind, 1, "affinityHash == sessionHash should only write once")
	require.Equal(t, hash, cache.setBind[0].sessionKey)
}

// TestBindSessionAndAffinity_SkipsWhenSessionHashEmpty 验证 sessionHash 为空时不写入。
func TestBindSessionAndAffinity_SkipsWhenSessionHashEmpty(t *testing.T) {
	cache := &mockCache{}
	svc := buildSvcWithCache(cache)

	ctx := ctxWithAffinity("some-affinity")
	gid := int64(1)
	svc.bindSessionAndAffinity(ctx, &gid, "", 42)

	cache.mu.Lock()
	defer cache.mu.Unlock()
	require.Empty(t, cache.setBind, "empty sessionHash should not write any binding")
}

// ============ refreshSessionAndAffinity 测试 ============

// TestRefreshSessionAndAffinity_RefreshesSessionAndAffinity 验证同时刷新两个 TTL。
func TestRefreshSessionAndAffinity_RefreshesSessionAndAffinity(t *testing.T) {
	cache := &mockCache{}
	svc := buildSvcWithCache(cache)

	sessionHash := "session-333"
	affinityHash := "affinity-444"
	ctx := ctxWithAffinity(affinityHash)
	gid := int64(2)

	svc.refreshSessionAndAffinity(ctx, &gid, sessionHash, int64(99))

	cache.mu.Lock()
	defer cache.mu.Unlock()

	require.Len(t, cache.refresh, 2, "should refresh both session TTL and affinity TTL")
	keys := map[string]bool{}
	for _, r := range cache.refresh {
		keys[r.sessionKey] = true
	}
	require.True(t, keys[sessionHash], "session hash TTL should be refreshed")
	require.True(t, keys[affinityHash], "affinity hash TTL should be refreshed")
}

// TestRefreshSessionAndAffinity_SkipsWhenSessionHashEmpty 验证 sessionHash 为空时不刷新。
func TestRefreshSessionAndAffinity_SkipsWhenSessionHashEmpty(t *testing.T) {
	cache := &mockCache{}
	svc := buildSvcWithCache(cache)

	ctx := ctxWithAffinity("some-affinity")
	gid := int64(1)
	svc.refreshSessionAndAffinity(ctx, &gid, "", int64(0))

	cache.mu.Lock()
	defer cache.mu.Unlock()
	require.Empty(t, cache.refresh, "empty sessionHash should not refresh anything")
}

// TestRefreshSessionAndAffinity_OnlySessionWhenNoAffinity 验证无 affinity hash 时只刷新 session。
func TestRefreshSessionAndAffinity_OnlySessionWhenNoAffinity(t *testing.T) {
	cache := &mockCache{}
	svc := buildSvcWithCache(cache)

	ctx := ctxWithAffinity("")
	gid := int64(1)
	svc.refreshSessionAndAffinity(ctx, &gid, "session-xyz", int64(42))

	cache.mu.Lock()
	defer cache.mu.Unlock()
	require.Len(t, cache.refresh, 1, "no affinity → only session TTL refreshed")
	require.Equal(t, "session-xyz", cache.refresh[0].sessionKey)
}
