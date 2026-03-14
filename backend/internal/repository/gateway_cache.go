package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	stickySessionPrefix        = "sticky_session:"
	stickyReversePrefix        = "sticky_reverse:"
)

type gatewayCache struct {
	rdb *redis.Client
}

func NewGatewayCache(rdb *redis.Client) service.GatewayCache {
	return &gatewayCache{rdb: rdb}
}

// buildSessionKey 构建 session key，包含 groupID 实现分组隔离
// 格式: sticky_session:{groupID}:{sessionHash}
func buildSessionKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%s%d:%s", stickySessionPrefix, groupID, sessionHash)
}

func (c *gatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Get(ctx, key).Int64()
}

func (c *gatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Set(ctx, key, accountID, ttl).Err()
}

func (c *gatewayCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// DeleteSessionAccountID 删除粘性会话与账号的绑定关系。
// 当检测到绑定的账号不可用（如状态错误、禁用、不可调度等）时调用，
// 以便下次请求能够重新选择可用账号。
//
// DeleteSessionAccountID removes the sticky session binding for the given session.
// Called when the bound account becomes unavailable (e.g., error status, disabled,
// or unschedulable), allowing subsequent requests to select a new available account.
func (c *gatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Del(ctx, key).Err()
}

func buildStickyReverseKey(accountID int64) string {
	return stickyReversePrefix + strconv.FormatInt(accountID, 10)
}

func buildStickyReverseMember(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%d:%s", groupID, sessionHash)
}

// AddStickySessionReverse 在反向索引中记录账号绑定的会话。
// 使用 Sorted Set，score 为过期时间戳，支持自清理。
func (c *gatewayCache) AddStickySessionReverse(ctx context.Context, accountID int64, groupID int64, sessionHash string, ttl time.Duration) error {
	key := buildStickyReverseKey(accountID)
	member := buildStickyReverseMember(groupID, sessionHash)
	expireAt := float64(time.Now().Add(ttl).Unix())
	if err := c.rdb.ZAdd(ctx, key, redis.Z{Score: expireAt, Member: member}).Err(); err != nil {
		return err
	}
	// 设置 key 级别的 EXPIRE 作为兜底，防止账号被删除后 key 永久留存
	c.rdb.Expire(ctx, key, ttl*2)
	return nil
}

// RemoveStickySessionReverse 从反向索引中删除会话绑定。
func (c *gatewayCache) RemoveStickySessionReverse(ctx context.Context, accountID int64, groupID int64, sessionHash string) error {
	key := buildStickyReverseKey(accountID)
	member := buildStickyReverseMember(groupID, sessionHash)
	return c.rdb.ZRem(ctx, key, member).Err()
}

// GetStickySessionCounts 批量查询每个账号的活跃粘性会话数。
// 先清理过期条目（score < now），再 ZCARD 计数。
func (c *gatewayCache) GetStickySessionCounts(ctx context.Context, accountIDs []int64) (map[int64]int, error) {
	if len(accountIDs) == 0 {
		return map[int64]int{}, nil
	}

	nowStr := strconv.FormatInt(time.Now().Unix(), 10)
	pipe := c.rdb.Pipeline()

	type acCmd struct {
		id       int64
		zcardCmd *redis.IntCmd
	}
	cmds := make([]acCmd, 0, len(accountIDs))
	for _, id := range accountIDs {
		key := buildStickyReverseKey(id)
		pipe.ZRemRangeByScore(ctx, key, "-inf", nowStr)
		cmds = append(cmds, acCmd{
			id:       id,
			zcardCmd: pipe.ZCard(ctx, key),
		})
	}

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("sticky reverse pipeline: %w", err)
	}

	result := make(map[int64]int, len(accountIDs))
	for _, ac := range cmds {
		result[ac.id] = int(ac.zcardCmd.Val())
	}
	return result, nil
}
