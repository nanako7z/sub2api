package service

import (
	"strconv"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// digestSessionTTL 直接引用 stickySessionTTL，确保两者始终一致。
// 过短的 TTL 会导致粘性会话 Redis 绑定还在但摘要链 fallback 已过期，
// 在 session hash 变化时无法通过摘要链回退到正确的账号。
const digestSessionTTL = stickySessionTTL

// sessionEntry flat cache 条目
type sessionEntry struct {
	uuid      string
	accountID int64
}

// DigestSessionStore 内存摘要会话存储（flat cache 实现）
// key: "{groupID}:{prefixHash}|{digestChain}" → *sessionEntry
type DigestSessionStore struct {
	cache *gocache.Cache
}

// NewDigestSessionStore 创建内存摘要会话存储
func NewDigestSessionStore() *DigestSessionStore {
	return &DigestSessionStore{
		cache: gocache.New(digestSessionTTL, time.Minute),
	}
}

// Save 保存摘要会话。oldDigestChain 为 Find 返回的 matchedChain，用于删旧 key。
func (s *DigestSessionStore) Save(groupID int64, prefixHash, digestChain, uuid string, accountID int64, oldDigestChain string) {
	if digestChain == "" {
		return
	}
	ns := buildNS(groupID, prefixHash)
	s.cache.Set(ns+digestChain, &sessionEntry{uuid: uuid, accountID: accountID}, gocache.DefaultExpiration)
	if oldDigestChain != "" && oldDigestChain != digestChain {
		s.cache.Delete(ns + oldDigestChain)
	}
}

// Find 查找摘要会话，从完整 chain 逐段截断，返回最长匹配及对应 matchedChain。
func (s *DigestSessionStore) Find(groupID int64, prefixHash, digestChain string) (uuid string, accountID int64, matchedChain string, found bool) {
	if digestChain == "" {
		return "", 0, "", false
	}
	ns := buildNS(groupID, prefixHash)
	chain := digestChain
	for {
		if val, ok := s.cache.Get(ns + chain); ok {
			if e, ok := val.(*sessionEntry); ok {
				return e.uuid, e.accountID, chain, true
			}
		}
		i := strings.LastIndex(chain, "-")
		if i < 0 {
			return "", 0, "", false
		}
		chain = chain[:i]
	}
}

// buildNS 构建 namespace 前缀
func buildNS(groupID int64, prefixHash string) string {
	return strconv.FormatInt(groupID, 10) + ":" + prefixHash + "|"
}
