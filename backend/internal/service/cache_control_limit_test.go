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

func TestEnforceCacheControlLimit_CleansThinkingEvenUnderLimit(t *testing.T) {
	// thinking 块有非法 cache_control，但总数 ≤ 4
	// 之前的 bug：early return 时 thinking 块的 cache_control 没有被清理
	data := map[string]any{
		"system": []any{
			map[string]any{"type": "thinking", "cache_control": map[string]any{"type": "ephemeral"}},
			map[string]any{"type": "text", "text": "sys", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "thinking", "thinking": "hmm", "cache_control": map[string]any{"type": "ephemeral"}},
					map[string]any{"type": "text", "text": "reply"},
				},
			},
		},
	}

	body, _ := json.Marshal(data)
	result := enforceCacheControlLimit(body)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result, &out))

	// thinking 块的 cache_control 应该被清理
	sysThinking := out["system"].([]any)[0].(map[string]any)
	require.NotContains(t, sysThinking, "cache_control", "thinking block in system should have cache_control removed")

	msgs := out["messages"].([]any)[0].(map[string]any)["content"].([]any)
	msgThinking := msgs[0].(map[string]any)
	require.NotContains(t, msgThinking, "cache_control", "thinking block in messages should have cache_control removed")

	// 非 thinking 块的 cache_control 应该保留
	sysText := out["system"].([]any)[1].(map[string]any)
	require.Contains(t, sysText, "cache_control", "text block cache_control should be preserved")
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

// ============ injectAutoCacheControl 测试 ============

func TestInjectAutoCacheControl_NoExistingCacheControl(t *testing.T) {
	// system 文本足够长（>1024 tokens ≈ >3072 chars for sonnet），无 cache_control
	longText := string(make([]byte, 4000)) // 4000 bytes → ~1333 tokens > 1024
	data := map[string]any{
		"system": []any{
			map[string]any{"type": "text", "text": longText},
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	body, _ := json.Marshal(data)
	result := injectAutoCacheControl(body, "claude-sonnet-4-6")

	var out map[string]any
	require.NoError(t, json.Unmarshal(result, &out))

	system := out["system"].([]any)
	lastBlock := system[len(system)-1].(map[string]any)
	require.Contains(t, lastBlock, "cache_control", "should inject cache_control on last system block")
}

func TestInjectAutoCacheControl_SkipsWhenClientHasCacheControl(t *testing.T) {
	longText := string(make([]byte, 4000))
	data := map[string]any{
		"system": []any{
			map[string]any{"type": "text", "text": longText, "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	body, _ := json.Marshal(data)
	result := injectAutoCacheControl(body, "claude-sonnet-4-6")

	// 不应修改（原封不动返回）
	require.Equal(t, body, result, "should not modify when client already has cache_control")
}

func TestInjectAutoCacheControl_SkipsWhenBelowTokenThreshold(t *testing.T) {
	// system 文本很短，低于 1024 token 阈值
	data := map[string]any{
		"system": []any{
			map[string]any{"type": "text", "text": "short prompt"},
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	body, _ := json.Marshal(data)
	result := injectAutoCacheControl(body, "claude-sonnet-4-6")

	var out map[string]any
	require.NoError(t, json.Unmarshal(result, &out))

	system := out["system"].([]any)
	lastBlock := system[0].(map[string]any)
	require.NotContains(t, lastBlock, "cache_control", "should not inject when below token threshold")
}

func TestInjectAutoCacheControl_HigherThresholdForOpus(t *testing.T) {
	// 2000 chars → ~666 tokens，高于 sonnet 阈值(1024) 但需 >4096 for opus
	// 实际：4000 chars → ~1333 tokens，仍低于 opus 4096
	mediumText := string(make([]byte, 4000))
	data := map[string]any{
		"system": []any{
			map[string]any{"type": "text", "text": mediumText},
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	body, _ := json.Marshal(data)
	result := injectAutoCacheControl(body, "claude-opus-4-6")

	var out map[string]any
	require.NoError(t, json.Unmarshal(result, &out))

	system := out["system"].([]any)
	lastBlock := system[0].(map[string]any)
	require.NotContains(t, lastBlock, "cache_control", "should not inject for opus when below 4096 token threshold")
}

func TestInjectAutoCacheControl_OpusAboveThreshold(t *testing.T) {
	// 13000 chars → ~4333 tokens > 4096
	longText := string(make([]byte, 13000))
	data := map[string]any{
		"system": []any{
			map[string]any{"type": "text", "text": longText},
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	body, _ := json.Marshal(data)
	result := injectAutoCacheControl(body, "claude-opus-4-5")

	var out map[string]any
	require.NoError(t, json.Unmarshal(result, &out))

	system := out["system"].([]any)
	lastBlock := system[0].(map[string]any)
	require.Contains(t, lastBlock, "cache_control", "should inject for opus when above 4096 token threshold")
}

func TestInjectAutoCacheControl_NoSystem(t *testing.T) {
	data := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	body, _ := json.Marshal(data)
	result := injectAutoCacheControl(body, "claude-sonnet-4-6")

	require.Equal(t, body, result, "should not modify when no system present")
}

func TestInjectAutoCacheControl_SkipsThinkingAsLastBlock(t *testing.T) {
	longText := string(make([]byte, 4000))
	data := map[string]any{
		"system": []any{
			map[string]any{"type": "text", "text": longText},
			map[string]any{"type": "thinking", "thinking": "internal thought"},
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	body, _ := json.Marshal(data)
	result := injectAutoCacheControl(body, "claude-sonnet-4-6")

	var out map[string]any
	require.NoError(t, json.Unmarshal(result, &out))

	system := out["system"].([]any)
	// 应注入到第一个 text 块（最后一个非 thinking 块），而非 thinking 块
	textBlock := system[0].(map[string]any)
	thinkingBlock := system[1].(map[string]any)
	require.Contains(t, textBlock, "cache_control", "should inject on last non-thinking block")
	require.NotContains(t, thinkingBlock, "cache_control", "should not inject on thinking block")
}

func TestInjectAutoCacheControl_ToolsContributeToTokenCount(t *testing.T) {
	// system 文本短，但 tools JSON 足够大，合计超过阈值
	tools := make([]any, 0, 20)
	for i := 0; i < 20; i++ {
		tools = append(tools, map[string]any{
			"name":        "tool_" + string(rune('a'+i)),
			"description": string(make([]byte, 200)), // 每个工具 ~200 chars
		})
	}

	data := map[string]any{
		"tools": tools,
		"system": []any{
			map[string]any{"type": "text", "text": "You are a helpful assistant."},
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	body, _ := json.Marshal(data)
	result := injectAutoCacheControl(body, "claude-sonnet-4-6")

	var out map[string]any
	require.NoError(t, json.Unmarshal(result, &out))

	system := out["system"].([]any)
	lastBlock := system[0].(map[string]any)
	require.Contains(t, lastBlock, "cache_control", "tools JSON should contribute to token count")
}

func TestMinCacheTokensByModel(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		{"claude-sonnet-4-6", 1024},
		{"claude-sonnet-4", 1024},
		{"claude-opus-4-6", 4096},
		{"claude-opus-4-5", 4096},
		{"claude-opus-4.5", 4096},
		{"claude-opus-4", 1024},
		{"claude-opus-4-1", 1024},
		{"claude-haiku-4-5", 4096},
		{"claude-haiku-3-5", 4096},
		{"claude-3-haiku-20240307", 4096},
		{"unknown-model", 1024},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			require.Equal(t, tt.expected, minCacheTokensByModel(tt.model))
		})
	}
}
