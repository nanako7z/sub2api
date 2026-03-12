//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// ============ countCacheControlBlocks 测试 ============

func TestCountCacheControlBlocks_Empty(t *testing.T) {
	data := map[string]any{}
	require.Equal(t, 0, countCacheControlBlocks(data))
}

func TestCountCacheControlBlocks_ToolsOnly(t *testing.T) {
	data := map[string]any{
		"tools": []any{
			map[string]any{"name": "tool1", "cache_control": map[string]any{"type": "ephemeral"}},
			map[string]any{"name": "tool2"},
			map[string]any{"name": "tool3", "cache_control": map[string]any{"type": "ephemeral"}},
		},
	}
	require.Equal(t, 2, countCacheControlBlocks(data))
}

func TestCountCacheControlBlocks_SystemOnly(t *testing.T) {
	data := map[string]any{
		"system": []any{
			map[string]any{"type": "text", "text": "hello", "cache_control": map[string]any{"type": "ephemeral"}},
			map[string]any{"type": "text", "text": "world"},
		},
	}
	require.Equal(t, 1, countCacheControlBlocks(data))
}

func TestCountCacheControlBlocks_MessagesOnly(t *testing.T) {
	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello", "cache_control": map[string]any{"type": "ephemeral"}},
				},
			},
		},
	}
	require.Equal(t, 1, countCacheControlBlocks(data))
}

func TestCountCacheControlBlocks_AllSections(t *testing.T) {
	data := map[string]any{
		"tools": []any{
			map[string]any{"name": "t1", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"system": []any{
			map[string]any{"type": "text", "text": "sys", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "msg", "cache_control": map[string]any{"type": "ephemeral"}},
				},
			},
		},
	}
	require.Equal(t, 3, countCacheControlBlocks(data))
}

func TestCountCacheControlBlocks_SkipsThinking(t *testing.T) {
	data := map[string]any{
		"system": []any{
			map[string]any{"type": "thinking", "cache_control": map[string]any{"type": "ephemeral"}},
			map[string]any{"type": "text", "text": "sys", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "thinking", "cache_control": map[string]any{"type": "ephemeral"}},
					map[string]any{"type": "text", "text": "reply", "cache_control": map[string]any{"type": "ephemeral"}},
				},
			},
		},
	}
	// thinking blocks should be skipped: 1 system text + 1 message text = 2
	require.Equal(t, 2, countCacheControlBlocks(data))
}

// ============ removeCacheControlFromTools 测试 ============

func TestRemoveCacheControlFromTools_Empty(t *testing.T) {
	data := map[string]any{}
	require.False(t, removeCacheControlFromTools(data))
}

func TestRemoveCacheControlFromTools_NoTools(t *testing.T) {
	data := map[string]any{"system": []any{}}
	require.False(t, removeCacheControlFromTools(data))
}

func TestRemoveCacheControlFromTools_RemovesLastFirst(t *testing.T) {
	data := map[string]any{
		"tools": []any{
			map[string]any{"name": "t1", "cache_control": map[string]any{"type": "ephemeral"}},
			map[string]any{"name": "t2"},
			map[string]any{"name": "t3", "cache_control": map[string]any{"type": "ephemeral"}},
		},
	}

	// 第一次移除：应移除最后一个（t3）的 cache_control
	require.True(t, removeCacheControlFromTools(data))
	tools := data["tools"].([]any)
	t1 := tools[0].(map[string]any)
	t3 := tools[2].(map[string]any)
	require.Contains(t, t1, "cache_control", "t1 should still have cache_control")
	require.NotContains(t, t3, "cache_control", "t3 should have cache_control removed")

	// 第二次移除：应移除 t1 的 cache_control
	require.True(t, removeCacheControlFromTools(data))
	require.NotContains(t, t1, "cache_control", "t1 should have cache_control removed")

	// 第三次：无可移除
	require.False(t, removeCacheControlFromTools(data))
}

// ============ enforceCacheControlLimit 移除顺序测试 ============

func TestEnforceCacheControlLimit_RemovesMessagesFirst(t *testing.T) {
	// 5 个 cache_control 块：1 tools + 1 system + 3 messages
	// 应先移除 messages 中的，保留 tools 和 system
	data := map[string]any{
		"tools": []any{
			map[string]any{"name": "t1", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"system": []any{
			map[string]any{"type": "text", "text": "sys", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "m1", "cache_control": map[string]any{"type": "ephemeral"}},
				},
			},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": "m2", "cache_control": map[string]any{"type": "ephemeral"}},
				},
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "m3", "cache_control": map[string]any{"type": "ephemeral"}},
				},
			},
		},
	}

	body, _ := json.Marshal(data)
	result := enforceCacheControlLimit(body)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result, &out))

	// tools 和 system 的 cache_control 应保留
	tools := out["tools"].([]any)
	require.Contains(t, tools[0].(map[string]any), "cache_control", "tools cache_control should be preserved")

	system := out["system"].([]any)
	require.Contains(t, system[0].(map[string]any), "cache_control", "system cache_control should be preserved")

	// 总数应 <= 4
	require.LessOrEqual(t, countCacheControlBlocks(out), 4)
}

func TestEnforceCacheControlLimit_PreservesToolsLast(t *testing.T) {
	// 6 个 cache_control 块：3 tools + 2 system + 1 messages = 6
	// 移除顺序：messages(1) → system(2) → tools(3)
	// 需要移除 2 个，最终剩 4 个
	data := map[string]any{
		"tools": []any{
			map[string]any{"name": "t1", "cache_control": map[string]any{"type": "ephemeral"}},
			map[string]any{"name": "t2", "cache_control": map[string]any{"type": "ephemeral"}},
			map[string]any{"name": "t3", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"system": []any{
			map[string]any{"type": "text", "text": "s1", "cache_control": map[string]any{"type": "ephemeral"}},
			map[string]any{"type": "text", "text": "s2", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "m1", "cache_control": map[string]any{"type": "ephemeral"}},
				},
			},
		},
	}

	body, _ := json.Marshal(data)
	result := enforceCacheControlLimit(body)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result, &out))

	count := countCacheControlBlocks(out)
	require.Equal(t, 4, count, "should reduce to exactly 4 cache_control blocks")

	// messages 应已被移除
	msgs := out["messages"].([]any)
	msg0 := msgs[0].(map[string]any)
	content := msg0["content"].([]any)
	require.NotContains(t, content[0].(map[string]any), "cache_control", "messages cache_control should be removed first")

	// tools 的前两个应保留（移除了 1 messages + 1 system = 2，tools 全保留）
	tools := out["tools"].([]any)
	require.Contains(t, tools[0].(map[string]any), "cache_control")
	require.Contains(t, tools[1].(map[string]any), "cache_control")
	require.Contains(t, tools[2].(map[string]any), "cache_control")
}

func TestEnforceCacheControlLimit_UnderLimitNoChange(t *testing.T) {
	data := map[string]any{
		"tools": []any{
			map[string]any{"name": "t1", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"system": []any{
			map[string]any{"type": "text", "text": "sys", "cache_control": map[string]any{"type": "ephemeral"}},
		},
	}

	body, _ := json.Marshal(data)
	result := enforceCacheControlLimit(body)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result, &out))
	require.Equal(t, 2, countCacheControlBlocks(out), "under limit, no blocks should be removed")
}

// ============ hasCacheControlEphemeral 测试 ============

func TestHasCacheControlEphemeral_True(t *testing.T) {
	block := map[string]any{
		"type":          "text",
		"text":          "hello",
		"cache_control": map[string]any{"type": "ephemeral"},
	}
	require.True(t, hasCacheControlEphemeral(block))
}

func TestHasCacheControlEphemeral_DifferentType(t *testing.T) {
	block := map[string]any{
		"type":          "text",
		"text":          "hello",
		"cache_control": map[string]any{"type": "5m"},
	}
	require.False(t, hasCacheControlEphemeral(block))
}

func TestHasCacheControlEphemeral_NoCacheControl(t *testing.T) {
	block := map[string]any{
		"type": "text",
		"text": "hello",
	}
	require.False(t, hasCacheControlEphemeral(block))
}

func TestHasCacheControlEphemeral_InvalidCacheControl(t *testing.T) {
	block := map[string]any{
		"type":          "text",
		"text":          "hello",
		"cache_control": "not-a-map",
	}
	require.False(t, hasCacheControlEphemeral(block))
}

// ============ removeCacheControlFromMessages 尾部优先测试 ============

func TestRemoveCacheControlFromMessages_TailFirst(t *testing.T) {
	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "first", "cache_control": map[string]any{"type": "ephemeral"}},
				},
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "last", "cache_control": map[string]any{"type": "ephemeral"}},
				},
			},
		},
	}

	// 第一次移除：应移除最后一条消息的 cache_control
	require.True(t, removeCacheControlFromMessages(data))

	msgs := data["messages"].([]any)
	first := msgs[0].(map[string]any)["content"].([]any)[0].(map[string]any)
	last := msgs[1].(map[string]any)["content"].([]any)[0].(map[string]any)

	require.Contains(t, first, "cache_control", "first message should still have cache_control")
	require.NotContains(t, last, "cache_control", "last message should have cache_control removed first")
}

func TestRemoveCacheControlFromMessages_SkipsThinking(t *testing.T) {
	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "thinking", "cache_control": map[string]any{"type": "ephemeral"}},
					map[string]any{"type": "text", "text": "reply", "cache_control": map[string]any{"type": "ephemeral"}},
				},
			},
		},
	}

	require.True(t, removeCacheControlFromMessages(data))

	content := data["messages"].([]any)[0].(map[string]any)["content"].([]any)
	thinking := content[0].(map[string]any)
	text := content[1].(map[string]any)

	require.Contains(t, thinking, "cache_control", "thinking blocks should not be touched")
	require.NotContains(t, text, "cache_control", "text block should have cache_control removed")
}
