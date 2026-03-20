# tlscapture — Claude Code TLS 指纹 & HTTP 请求头抓取工具

通过 hosts 劫持 + 自签名证书捕获服务器，抓取真实 Claude Code 客户端发送 API 请求时的 TLS ClientHello 指纹和 HTTP 请求头。

## 原理

```
远程机器 (Claude Code)                    本机 (捕获服务器)
┌──────────────────────┐                 ┌──────────────────────┐
│ claude -p "hi"       │                 │ TLS Capture Server   │
│                      │   TCP:443       │ 0.0.0.0:443          │
│ /etc/hosts:          │ ──────────────> │                      │
│ {localIP}            │                 │ GetConfigForClient() │
│   api.anthropic.com  │                 │  → 捕获 ClientHello  │
│                      │   TLS Handshake │                      │
│ NODE_TLS_REJECT_     │ <────────────── │ 自签名证书            │
│   UNAUTHORIZED=0     │                 │ SAN=api.anthropic.com│
│                      │   HTTP Request  │                      │
│ OAuth 已登录         │ ──────────────> │ HTTP Handler          │
│                      │                 │  → 捕获请求头         │
└──────────────────────┘                 └──────────────────────┘
```

**关键点：**
- hosts 劫持使 Claude Code 认为自己在连接真实的 `api.anthropic.com`，SNI 和 Host 头真实
- `NODE_TLS_REJECT_UNAUTHORIZED=0` 让 Bun/BoringSSL 跳过自签名证书验证
- Go `tls.Config.GetConfigForClient` 回调在 TLS 握手阶段捕获完整 ClientHello
- HTTP handler 捕获请求头后返回 503 overloaded_error，防止 Claude Code 无限重试

## 前置条件

1. **远程机器**：已安装 Claude Code 并完成 OAuth 登录
2. **SSH 访问**：远程用户可以使用 `sudo`
3. **网络互通**：本机 IP 可从远程机器访问
4. **本机端口 443**：未被占用，防火墙允许入站

## 配置

编辑 `main.go` 顶部常量：

```go
const (
    listenAddr = "0.0.0.0:443"
    remoteHost = "192.168.50.102:22"  // 远程机器 SSH
    remoteUser = "nanako"
    remotePass = "djl0629@nA"
    localIP    = "192.168.16.107"     // 本机可达 IP
)
```

## 运行

### 1. 开放本机防火墙 443 端口（管理员 PowerShell）

```powershell
New-NetFirewallRule -DisplayName "TLS Capture 443" -Direction Inbound -LocalPort 443 -Protocol TCP -Action Allow
```

### 2. 运行捕获工具

```bash
cd backend
go run ./cmd/tlscapture/
```

### 3. 自动执行流程

```
[1/5] 启动本机 TLS 捕获服务器（0.0.0.0:443）
[2/5] SSH 连接远程机器
[3/5] sudo 劫持 /etc/hosts → {localIP} api.anthropic.com
[4/5] 远程执行 NODE_TLS_REJECT_UNAUTHORIZED=0 claude -p "hi"
[5/5] 还原 /etc/hosts，输出对比结果
```

### 4. 清理防火墙规则（管理员 PowerShell）

```powershell
Remove-NetFirewallRule -DisplayName "TLS Capture 443"
```

## 输出示例（2026-03-20，Claude Code v2.1.80）

### TLS 指纹

| 项目 | 值 |
|------|---|
| SNI | `api.anthropic.com` |
| TLS Version | 0x0303 (TLS 1.2 record) + supported_versions: TLS 1.3, TLS 1.2 |
| Cipher Suites (17) | AES-GCM, ChaCha20, ECDHE 系列，无 GREASE |
| Curves (3) | X25519, P-256, P-384 |
| Point Formats | uncompressed (0) |
| ALPN | `http/1.1` only（无 h2） |
| Signature Algorithms (9) | ECDSA P256/P384, RSA-PSS, RSA-PKCS1, legacy SHA1 |
| GREASE | 无（Bun/BoringSSL 不使用） |

### HTTP 请求头（/v1/messages）

| Header | 值 |
|--------|---|
| User-Agent | `claude-cli/2.1.80 (external, sdk-cli)` |
| X-Stainless-Runtime | `node` |
| X-Stainless-Runtime-Version | `v24.3.0` |
| X-Stainless-Package-Version | `0.74.0` |
| X-Stainless-Os | `Linux` |
| X-Stainless-Arch | `x64` |
| X-Stainless-Lang | `js` |
| X-Stainless-Retry-Count | `0` |
| X-Stainless-Timeout | `600` |
| X-App | `cli` |
| Accept-Encoding | `gzip, deflate, br, zstd` |
| Anthropic-Dangerous-Direct-Browser-Access | `true` |
| Anthropic-Version | `2023-06-01` |
| Anthropic-Beta | `claude-code-20250219,oauth-2025-04-20,context-1m-2025-08-07,interleaved-thinking-2025-05-14,context-management-2025-06-27,prompt-caching-scope-2026-01-05,advanced-tool-use-2025-11-20,effort-2025-11-24` |

### 其他接口请求头差异

Claude Code 启动时会发多个请求，使用不同的 User-Agent：

| 接口 | User-Agent |
|------|-----------|
| `/v1/messages` | `claude-cli/2.1.80 (external, sdk-cli)` |
| `/v1/mcp_servers` | `axios/1.13.6` |
| `/api/eval/sdk-*` | `Bun/1.3.11` |
| `/api/oauth/claude_cli/client_data` | `claude-code/2.1.80` |
| `/api/claude_code_grove` | `claude-cli/2.1.80 (external, sdk-cli)` |
| 其他 `/api/*` | `axios/1.13.6` |

## 更新指纹后需修改的文件

捕获到新指纹后，对比输出会显示差异。需要更新：

| 文件 | 内容 |
|------|------|
| `internal/pkg/tlsfingerprint/dialer.go` | `defaultCipherSuites`、`defaultCurves`、`defaultSignatureAlgorithms`、ALPN、扩展顺序 |
| `internal/pkg/tlsfingerprint/registry.go` | Profile 名称中的版本号 |
| `internal/pkg/claude/constants.go` | `DefaultHeaders`（User-Agent 版本号等）、Beta 常量 |
| `internal/pkg/tlsfingerprint/dialer_integration_test.go` | 预期的 JA3/JA4 哈希值 |
