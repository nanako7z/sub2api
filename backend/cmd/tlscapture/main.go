// Command tlscapture is a legacy one-shot calibration tool that starts a local TLS
// capture server and remotely triggers Claude Code to connect to it, capturing a
// single real ClientHello fingerprint and HTTP request header snapshot.
//
// Prefer tools/mitm for continuous observation. tlscapture remains useful when a
// quick remote TLS/header validation is needed and proxy-based MITM is unavailable.
//
// It hijacks /etc/hosts on the remote machine to redirect api.anthropic.com to the
// local capture server, then triggers one Claude Code API request.
//
// Usage: go run ./cmd/tlscapture
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// ========== Configuration ==========

// Command-line flags (override via env: CAPTURE_REMOTE_HOST, CAPTURE_REMOTE_USER,
// CAPTURE_REMOTE_PASS, CAPTURE_LOCAL_IP)
var (
	flagRemoteHost = flag.String("remote", envOrDefault("CAPTURE_REMOTE_HOST", "192.168.50.102:22"), "Remote SSH host:port")
	flagRemoteUser = flag.String("user", envOrDefault("CAPTURE_REMOTE_USER", ""), "Remote SSH username")
	flagRemotePass = flag.String("pass", envOrDefault("CAPTURE_REMOTE_PASS", ""), "Remote SSH password")
	flagLocalIP    = flag.String("local-ip", envOrDefault("CAPTURE_LOCAL_IP", ""), "This machine's IP reachable from remote")
	flagListenAddr = flag.String("listen", "0.0.0.0:443", "Capture server listen address")
)

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

const hostsMarker = "# tlscapture-temp"

// ========== TLS ClientHello Capture ==========

// CapturedFingerprint holds all extracted TLS fingerprint data.
type CapturedFingerprint struct {
	TLSVersion     uint16
	CipherSuites   []uint16
	Extensions     []uint16
	Curves         []uint16 // Supported groups
	PointFormats   []uint8
	ALPNProtocols  []string
	ServerName     string
	SignatureAlgos []uint16
	TLSVersions    []uint16 // supported_versions extension
}

// CapturedRequest holds HTTP-level info.
type CapturedRequest struct {
	Method  string
	URL     string
	Headers http.Header
}

var (
	capturedFP  *CapturedFingerprint
	capturedReq *CapturedRequest
	captureMu   sync.Mutex
	captureDone = make(chan struct{})
	captureOnce sync.Once
)

// signalCaptureDone marks capture as complete.
func signalCaptureDone() {
	captureOnce.Do(func() { close(captureDone) })
}

// formatCipherSuites returns hex-formatted cipher suite list.
func formatCipherSuites(suites []uint16) string {
	parts := make([]string, len(suites))
	for i, s := range suites {
		parts[i] = fmt.Sprintf("0x%04x", s)
	}
	return strings.Join(parts, ", ")
}

// formatCipherSuitesDecimal returns decimal-formatted (JA3 style).
func formatCipherSuitesDecimal(suites []uint16) string {
	parts := make([]string, len(suites))
	for i, s := range suites {
		parts[i] = fmt.Sprintf("%d", s)
	}
	return strings.Join(parts, "-")
}

// ========== Self-Signed Certificate ==========

func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "api.anthropic.com"},
		DNSNames:     []string{"api.anthropic.com", "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP(*flagLocalIP)},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return tls.X509KeyPair(certPEM, keyPEM)
}

// ========== TLS Capture Server ==========

func startCaptureServer() (*http.Server, error) {
	cert, err := generateSelfSignedCert()
	if err != nil {
		return nil, fmt.Errorf("generate cert: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			captureMu.Lock()
			defer captureMu.Unlock()

			fp := &CapturedFingerprint{
				TLSVersion:   0x0303, // ClientHelloInfo doesn't expose record version; use TLS 1.2 as default
				CipherSuites: make([]uint16, len(hello.CipherSuites)),
				ServerName:   hello.ServerName,
			}
			copy(fp.CipherSuites, hello.CipherSuites)

			// Supported curves (groups)
			for _, c := range hello.SupportedCurves {
				fp.Curves = append(fp.Curves, uint16(c))
			}

			// Point formats
			fp.PointFormats = append(fp.PointFormats, hello.SupportedPoints...)

			// ALPN protocols
			fp.ALPNProtocols = append(fp.ALPNProtocols, hello.SupportedProtos...)

			// Supported versions
			for _, v := range hello.SupportedVersions {
				fp.TLSVersions = append(fp.TLSVersions, v)
			}

			// Signature algorithms
			for _, s := range hello.SignatureSchemes {
				fp.SignatureAlgos = append(fp.SignatureAlgos, uint16(s))
			}

			capturedFP = fp

			log.Println("========== ClientHello Captured ==========")
			log.Printf("SNI:             %s", fp.ServerName)
			log.Printf("Version:         0x%04x", fp.TLSVersion)
			log.Printf("CipherSuites(%d): %s", len(fp.CipherSuites), formatCipherSuites(fp.CipherSuites))
			log.Printf("Curves(%d):       %s", len(fp.Curves), formatCipherSuites(fp.Curves))
			log.Printf("PointFormats(%d): %v", len(fp.PointFormats), fp.PointFormats)
			log.Printf("ALPN:            %v", fp.ALPNProtocols)
			log.Printf("SigAlgos(%d):     %s", len(fp.SignatureAlgos), formatCipherSuites(fp.SignatureAlgos))
			log.Printf("SupportedVers:   %s", formatCipherSuites(fp.TLSVersions))
			log.Println("===========================================")

			return nil, nil
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		captureMu.Lock()
		capturedReq = &CapturedRequest{
			Method:  r.Method,
			URL:     r.URL.String(),
			Headers: r.Header.Clone(),
		}
		captureMu.Unlock()

		log.Println("========== HTTP Request Captured ==========")
		log.Printf("Method: %s", r.Method)
		log.Printf("URL:    %s", r.URL.String())
		log.Printf("Host:   %s", r.Host)
		for name, vals := range r.Header {
			for _, v := range vals {
				log.Printf("Header: %s: %s", name, v)
			}
		}
		log.Println("============================================")

		// Return a plausible Anthropic-like error so Claude Code doesn't retry endlessly
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"TLS capture complete"}}`))

		signalCaptureDone()
	})

	srv := &http.Server{
		Addr:      *flagListenAddr,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	ln, err := tls.Listen("tcp", *flagListenAddr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	go func() {
		log.Printf("TLS capture server listening on %s", *flagListenAddr)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	return srv, nil
}

// ========== SSH Remote Operations ==========

func sshConnect() (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: *flagRemoteUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(*flagRemotePass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", *flagRemoteHost, config)
	if err != nil {
		return nil, fmt.Errorf("SSH dial: %w", err)
	}
	return client, nil
}

func sshRun(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}

// ========== Hosts File Operations ==========

// sshRunSudo runs a command with sudo, piping the password via stdin.
func sshRunSudo(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	// Pipe password to sudo -S via bash -c with double quotes
	// to avoid single-quote nesting issues
	sess.Stdin = strings.NewReader(*flagRemotePass + "\n")
	fullCmd := fmt.Sprintf(`sudo -S bash -c "%s"`, strings.ReplaceAll(cmd, `"`, `\"`))
	out, err := sess.CombinedOutput(fullCmd)
	// Filter out the sudo password prompt from output
	result := string(out)
	lines := strings.Split(result, "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.Contains(line, "[sudo]") && !strings.Contains(line, "password") {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n"), err
}

// addHostsEntry appends the hijack entry to /etc/hosts on the remote machine.
func addHostsEntry(client *ssh.Client, entry string) error {
	cmd := fmt.Sprintf(`echo "%s" >> /etc/hosts`, entry)
	out, err := sshRunSudo(client, cmd)
	if err != nil {
		return fmt.Errorf("add hosts entry: %v\n%s", err, out)
	}
	return nil
}

// removeHostsEntry removes all lines with the marker from /etc/hosts.
func removeHostsEntry(client *ssh.Client) error {
	// Use grep -v to filter out marker lines, write to temp then move back
	cmd := fmt.Sprintf(`grep -v '%s' /etc/hosts > /tmp/hosts.clean && mv /tmp/hosts.clean /etc/hosts`, hostsMarker)
	out, err := sshRunSudo(client, cmd)
	if err != nil {
		return fmt.Errorf("remove hosts entry: %v\n%s", err, out)
	}
	return nil
}

// verifyHostsEntry checks that api.anthropic.com resolves to our IP.
func verifyHostsEntry(client *ssh.Client) bool {
	out, err := sshRun(client, fmt.Sprintf(`getent hosts api.anthropic.com | grep -q '%s' && echo OK || echo FAIL`, *flagLocalIP))
	return err == nil && strings.TrimSpace(out) == "OK"
}

// ========== Comparison Logic ==========

func printComparison() {
	captureMu.Lock()
	fp := capturedFP
	req := capturedReq
	captureMu.Unlock()

	if fp == nil {
		log.Println("No fingerprint captured!")
		return
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║            TLS 指纹对比：真实 Claude Code vs 当前伪装           ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// ---- Cipher Suites ----
	fmt.Println("┌─── Cipher Suites ───────────────────────────────────────────┐")
	fmt.Printf("│ 真实 Claude Code (%d 个):\n", len(fp.CipherSuites))
	// Filter out GREASE values
	realSuites := filterGREASE(fp.CipherSuites)
	fmt.Printf("│   %s\n", formatCipherSuites(realSuites))
	fmt.Printf("│\n")

	// Current emulation (from dialer.go defaults)
	emuSuites := []uint16{
		0x1301, 0x1302, 0x1303,
		0xc02b, 0xc02f, 0xc02c, 0xc030,
		0xcca9, 0xcca8,
		0xc009, 0xc013, 0xc00a, 0xc014,
		0x009c, 0x009d, 0x002f, 0x0035,
	}
	fmt.Printf("│ 当前伪装 (%d 个):\n", len(emuSuites))
	fmt.Printf("│   %s\n", formatCipherSuites(emuSuites))
	match := compareSuites(realSuites, emuSuites)
	fmt.Printf("│ 匹配: %s\n", match)
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// ---- Curves ----
	fmt.Println("┌─── Supported Groups (Curves) ──────────────────────────────┐")
	realCurves := filterGREASE(fp.Curves)
	fmt.Printf("│ 真实: %s\n", formatCipherSuites(realCurves))
	emuCurves := []uint16{0x001d, 0x0017, 0x0018}
	fmt.Printf("│ 伪装: %s\n", formatCipherSuites(emuCurves))
	fmt.Printf("│ 匹配: %s\n", compareSuites(realCurves, emuCurves))
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// ---- Point Formats ----
	fmt.Println("┌─── Point Formats ──────────────────────────────────────────┐")
	fmt.Printf("│ 真实: %v\n", fp.PointFormats)
	fmt.Printf("│ 伪装: [0]\n")
	pfMatch := "✅ 完全一致"
	if len(fp.PointFormats) != 1 || (len(fp.PointFormats) > 0 && fp.PointFormats[0] != 0) {
		pfMatch = "❌ 不一致"
	}
	fmt.Printf("│ 匹配: %s\n", pfMatch)
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// ---- ALPN ----
	fmt.Println("┌─── ALPN ───────────────────────────────────────────────────┐")
	fmt.Printf("│ 真实: %v\n", fp.ALPNProtocols)
	fmt.Printf("│ 伪装: [http/1.1]\n")
	alpnMatch := "✅ 完全一致"
	if len(fp.ALPNProtocols) != 1 || (len(fp.ALPNProtocols) >= 1 && fp.ALPNProtocols[0] != "http/1.1") {
		alpnMatch = "❌ 不一致"
	}
	fmt.Printf("│ 匹配: %s\n", alpnMatch)
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// ---- Signature Algorithms ----
	fmt.Println("┌─── Signature Algorithms ───────────────────────────────────┐")
	fmt.Printf("│ 真实 (%d 个): %s\n", len(fp.SignatureAlgos), formatCipherSuites(fp.SignatureAlgos))
	emuSigs := []uint16{0x0403, 0x0804, 0x0401, 0x0503, 0x0805, 0x0501, 0x0806, 0x0601, 0x0201}
	fmt.Printf("│ 伪装 (%d 个): %s\n", len(emuSigs), formatCipherSuites(emuSigs))
	fmt.Printf("│ 匹配: %s\n", compareSuites(fp.SignatureAlgos, emuSigs))
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// ---- Supported Versions ----
	fmt.Println("┌─── Supported TLS Versions ─────────────────────────────────┐")
	realVers := filterGREASE(fp.TLSVersions)
	fmt.Printf("│ 真实: %s\n", formatCipherSuites(realVers))
	fmt.Printf("│ 伪装: 0x0304, 0x0303 (TLS 1.3, TLS 1.2)\n")
	fmt.Printf("│ 匹配: %s\n", compareSuites(realVers, []uint16{0x0304, 0x0303}))
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// ---- HTTP Headers ----
	if req != nil {
		fmt.Println("┌─── HTTP Headers ───────────────────────────────────────────┐")
		fmt.Printf("│ Method: %s\n", req.Method)
		fmt.Printf("│ URL:    %s\n", req.URL)
		for name, vals := range req.Headers {
			for _, v := range vals {
				// Truncate long values
				if len(v) > 80 {
					v = v[:80] + "..."
				}
				fmt.Printf("│ %s: %s\n", name, v)
			}
		}
		fmt.Println("└─────────────────────────────────────────────────────────────┘")
	}

	// ---- JA3-like string ----
	fmt.Println()
	fmt.Println("┌─── JA3-style Fingerprint ──────────────────────────────────┐")
	ja3Real := fmt.Sprintf("%d,%s,,%s,%s",
		fp.TLSVersion,
		formatCipherSuitesDecimal(realSuites),
		formatCipherSuitesDecimal(realCurves),
		formatPointFormatsDecimal(fp.PointFormats),
	)
	fmt.Printf("│ 真实: %s\n", ja3Real)
	fmt.Println("└─────────────────────────────────────────────────────────────┘")

	// ---- Raw hex dump for extension analysis ----
	fmt.Println()
	fmt.Println("┌─── 原始数据（可用于更新 dialer.go）─────────────────────────┐")
	fmt.Printf("│ CipherSuites: []uint16{%s}\n", formatGoSlice(realSuites))
	fmt.Printf("│ Curves:       []uint16{%s}\n", formatGoSlice(realCurves))
	fmt.Printf("│ PointFormats: []uint8{%s}\n", formatGoSliceU8(fp.PointFormats))
	fmt.Printf("│ SigAlgos:     []uint16{%s}\n", formatGoSlice(fp.SignatureAlgos))
	fmt.Printf("│ ALPN:         %v\n", fp.ALPNProtocols)
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
}

func filterGREASE(vals []uint16) []uint16 {
	var out []uint16
	for _, v := range vals {
		if !isGREASE(v) {
			out = append(out, v)
		}
	}
	return out
}

func isGREASE(v uint16) bool {
	// GREASE values: 0x0a0a, 0x1a1a, 0x2a2a, ..., 0xfafa
	return (v & 0x0f0f) == 0x0a0a
}

func compareSuites(real, emulated []uint16) string {
	if len(real) != len(emulated) {
		return fmt.Sprintf("❌ 数量不同 (真实=%d, 伪装=%d)", len(real), len(emulated))
	}
	for i := range real {
		if real[i] != emulated[i] {
			return fmt.Sprintf("❌ 第 %d 个不同 (真实=0x%04x, 伪装=0x%04x)", i+1, real[i], emulated[i])
		}
	}
	return "✅ 完全一致"
}

func formatGoSlice(vals []uint16) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("0x%04x", v)
	}
	return strings.Join(parts, ", ")
}

func formatGoSliceU8(vals []uint8) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, ", ")
}

func formatPointFormatsDecimal(vals []uint8) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, "-")
}

// ========== Main ==========

func main() {
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// Validate required flags
	if *flagRemoteUser == "" || *flagRemotePass == "" || *flagLocalIP == "" {
		fmt.Fprintln(os.Stderr, "Usage: tlscapture -user USER -pass PASS -local-ip IP [-remote HOST:PORT] [-listen ADDR]")
		fmt.Fprintln(os.Stderr, "Legacy one-shot TLS/header calibration tool. Prefer tools/mitm for continuous capture.")
		fmt.Fprintln(os.Stderr, "  Or set env: CAPTURE_REMOTE_USER, CAPTURE_REMOTE_PASS, CAPTURE_LOCAL_IP")
		flag.PrintDefaults()
		os.Exit(1)
	}

	hostsEntry := *flagLocalIP + " api.anthropic.com " + hostsMarker

	// Handle Ctrl+C for cleanup
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	// Step 1: Start TLS capture server
	log.Printf("[1/5] 启动 TLS 捕获服务器 (%s)...", *flagListenAddr)
	srv, err := startCaptureServer()
	if err != nil {
		log.Fatalf("Failed to start capture server: %v", err)
	}
	defer srv.Close()

	// Step 2: SSH connect
	log.Println("[2/5] SSH 连接到远程机器...")
	client, err := sshConnect()
	if err != nil {
		log.Fatalf("SSH connect failed: %v", err)
	}
	defer client.Close()
	log.Println("  SSH 连接成功")

	// Get Claude Code version info
	if out, err := sshRun(client, "claude --version 2>/dev/null || echo 'unknown'"); err == nil {
		log.Printf("  Claude Code version: %s", strings.TrimSpace(out))
	}

	// Step 3: Hijack /etc/hosts
	log.Println("[3/5] 劫持 /etc/hosts: api.anthropic.com ->", *flagLocalIP)

	// Ensure cleanup on exit (idempotent via sync.Once)
	var hostsCleanupOnce sync.Once
	cleanupHosts := func() {
		hostsCleanupOnce.Do(func() {
			log.Println("  清理 /etc/hosts...")
			if err := removeHostsEntry(client); err != nil {
				log.Printf("  ⚠️ 清理失败: %v", err)
				log.Printf("  请手动执行: sudo sed -i '/%s/d' /etc/hosts", hostsMarker)
			} else {
				log.Println("  ✅ /etc/hosts 已恢复")
			}
		})
	}
	defer cleanupHosts()

	// First, remove any stale entries from previous runs
	removeHostsEntry(client)

	// Add the hijack entry
	if err := addHostsEntry(client, hostsEntry); err != nil {
		log.Fatalf("劫持 /etc/hosts 失败: %v", err)
	}

	// Verify
	if verifyHostsEntry(client) {
		log.Println("  ✅ hosts 劫持生效: api.anthropic.com ->", *flagLocalIP)
	} else {
		log.Println("  ⚠️ hosts 劫持可能未生效，继续尝试...")
	}

	// Show current hosts entry
	if out, err := sshRun(client, "grep api.anthropic.com /etc/hosts"); err == nil {
		log.Printf("  /etc/hosts 相关条目:\n%s", strings.TrimSpace(out))
	}

	// Step 4: Trigger Claude Code request
	log.Println("[4/5] 触发 Claude Code 请求...")
	log.Println("  OAuth 已登录，直接发送 API 请求...")

	go func() {
		// Check runtime info
		runtimeCheck, _ := sshRun(client, `ls -la $(which claude 2>/dev/null) 2>/dev/null; file $(readlink -f $(which claude 2>/dev/null)) 2>/dev/null`)
		log.Printf("  运行时环境:\n%s", strings.TrimSpace(runtimeCheck))

		// Invoke Claude Code with NODE_TLS_REJECT_UNAUTHORIZED=0 for self-signed cert
		// No need for ANTHROPIC_BASE_URL or ANTHROPIC_API_KEY - OAuth is already configured
		log.Println("  调用 Claude Code (NODE_TLS_REJECT_UNAUTHORIZED=0)...")
		claudeCmd := `NODE_TLS_REJECT_UNAUTHORIZED=0 timeout 15 claude -p "hi" 2>&1; true`
		out, _ := sshRun(client, claudeCmd)
		log.Printf("  Claude Code 输出: %s", strings.TrimSpace(out))
	}()

	// Wait for capture or timeout
	select {
	case <-captureDone:
		log.Println("  ✅ 指纹捕获成功！")
	case <-time.After(60 * time.Second):
		log.Println("  ❌ 捕获超时 (60s)")
	case <-sigCh:
		log.Println("  中断，清理中...")
	}

	// Step 5: Restore /etc/hosts (handled by defer cleanupHosts)
	log.Println("[5/5] 还原 /etc/hosts...")
	// cleanupHosts is called via defer, but we call it explicitly here
	// so the comparison output comes after cleanup
	cleanupHosts()

	// Verify restoration
	out, _ := sshRun(client, "grep api.anthropic.com /etc/hosts 2>/dev/null")
	if strings.TrimSpace(out) == "" {
		log.Println("  ✅ /etc/hosts 已完全恢复")
	} else {
		log.Printf("  ⚠️ /etc/hosts 仍有残留:\n%s", strings.TrimSpace(out))
	}

	// Print comparison
	printComparison()

	// Print detailed cipher suite list
	if capturedFP != nil {
		captureMu.Lock()
		raw := capturedFP
		captureMu.Unlock()
		realSuites := filterGREASE(raw.CipherSuites)
		fmt.Printf("\n完整 CipherSuites (非 GREASE, 共 %d 个):\n", len(realSuites))
		for i, s := range realSuites {
			fmt.Printf("  [%2d] 0x%04x (%d)\n", i, s, s)
		}
	}
}
