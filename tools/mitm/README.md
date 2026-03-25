# Claude Code MITM 真实请求观测工具链

这套工具用于在本机长期观察 Claude Code 的真实请求行为，替代 `backend/cmd/tlscapture` 作为 `sub2api` 的主请求捕获工作流。

它会通过 `mitmproxy/mitmdump` 记录完整请求生命周期，而不是只抓一次 TLS 和请求头。事件会写入 `captures/capture-*.jsonl`，分析结果写入 `reports/*.json`。

## 目标

- 观察多个真实端点，而不是只抓 `/v1/messages`
- 记录 TLS ClientHello、请求头、请求体、响应体
- 记录失败但没有响应的请求
- 区分 OAuth、自定义 `baseURL + API key` 等模式
- 为 `internal/pkg/tlsfingerprint/dialer.go` 和 `internal/pkg/claude/constants.go` 提供结构化事实来源

## 依赖

```bash
python3 -m pip install -r tools/mitm/requirements.txt
```

额外建议安装：

- `tmux`
- `mitmproxy` 的 CA 证书已导入本机

## 常用工作流

### 1. 启动常驻 MITM

```bash
MITM_PORT=8083 tools/mitm/start_daemon.sh
```

如果你本机已经设置了 `HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY`，脚本会优先继承它们作为 MITM 的上游出口。最稳妥的方式仍然是显式设置 `UPSTREAM_PROXY`，这样不会受当前 shell 里其他代理变量影响：

```bash
UPSTREAM_PROXY=http://127.0.0.1:7897 MITM_PORT=8083 tools/mitm/start_daemon.sh
```

脚本会自动忽略指向当前 MITM 监听端口的代理，避免把 `127.0.0.1:$MITM_PORT` 误当成上游出口形成代理环路。

### 2. 通过代理启动 Claude Code

```bash
MITM_PORT=8083 tools/mitm/claude_with_proxy.sh claude
```

也可以直接发测试请求：

```bash
MITM_PORT=8083 tools/mitm/claude_with_proxy.sh claude -p "hello"
```

如果使用前台模式，也会沿用相同的上游代理检测逻辑：

```bash
UPSTREAM_PROXY=http://127.0.0.1:7897 MITM_PORT=8083 tools/mitm/run.sh claude -p "hello"
```

### 3. 查看状态 / 停止

```bash
MITM_PORT=8083 tools/mitm/status.sh
MITM_PORT=8083 tools/mitm/stop_daemon.sh
```

### 4. 生成分析报告

```bash
python3 tools/mitm/analyze.py
```

### 5. 生成 Claude 可执行文件逆向证据报告（静态+动态）

```bash
python3 tools/mitm/build_reverse_report.py \
  --binary /Users/luli/.local/share/claude/versions/2.1.79 \
  --capture tools/mitm/captures/capture-1774444271.jsonl
```

输出：

- `reports/reverse_evidence_2.1.79.json`
- `reports/reverse_report_2.1.79.md`

### 6. 基线 vs 新回放 命中率对比

```bash
python3 tools/mitm/compare_captures.py \
  --baseline tools/mitm/captures/capture-1774444271.jsonl \
  --candidate tools/mitm/captures/<new-capture>.jsonl
```

输出：

- `reports/capture_compare_report.json`
- `reports/capture_compare_report.md`

## 事件模型

每个 flow 会按 `flow_id` 输出多条事件：

- `tls_clienthello`
- `request_headers`
- `request`
- `response`
- `error`

这样即使请求失败、超时、没有响应，也能在 `capture-*.jsonl` 中留下可分析记录。

## 报告

默认输出：

- `reports/hosts.json`
- `reports/endpoints.json`
- `reports/headers.json`
- `reports/tls_profiles.json`
- `reports/status_codes.json`
- `reports/errors.json`
- `reports/sse_events.json`
- `reports/nonessential_traffic.json`
- `reports/mode_diff_oauth_vs_apikey.json`
- `reports/geo_env_diff.json`

## 与 tlscapture 的关系

- `tools/mitm/`：主工作流，适合持续观测和行为分析
- `backend/cmd/tlscapture`：补充校准工具，适合无法使用代理时抓一次 TLS 和请求头
