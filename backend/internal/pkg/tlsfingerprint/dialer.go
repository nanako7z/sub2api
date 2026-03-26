// Package tlsfingerprint provides TLS fingerprint simulation for HTTP clients.
// It uses the utls library to create TLS connections that mimic Node.js/Claude Code clients.
package tlsfingerprint

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/proxy"
)

// Profile contains TLS fingerprint configuration.
type Profile struct {
	Name      string // Profile name for identification
	Mode      string // npm
	ShapeMode string // npm (legacy field, ignored by npm mode)
	// ShapeWindowSize controls rolling-window scheduling for observed_v1 mode.
	ShapeWindowSize int
	// ShapeWeights is a legacy field kept for compatibility.
	ShapeWeights   TLSShapeWeights
	ECHPayloadMode string // npm (legacy field, ignored by npm mode)
	CipherSuites   []uint16
	Curves         []uint16
	PointFormats   []uint8
	EnableGREASE   bool
	ECHConfigList  []byte // Optional ECHConfigList for endpoint-aware dynamic 65037 generation
	// ECHScopeKey is a runtime-only key used for endpoint-aware shape scheduling/cache segregation.
	ECHScopeKey string
	// ClientCacheKey is a runtime-only cache key suffix used by upstream client cache.
	ClientCacheKey string
}

type TLSShapeWeights struct {
	ECH186Padding41 int
	ECH218Padding9  int
	ECH250Padding0  int
	ECH282Padding0  int
}

// Dialer creates TLS connections with custom fingerprints.
type Dialer struct {
	profile    *Profile
	baseDialer func(ctx context.Context, network, addr string) (net.Conn, error)
}

// HTTPProxyDialer creates TLS connections through HTTP/HTTPS proxies with custom fingerprints.
// It handles the CONNECT tunnel establishment before performing TLS handshake.
type HTTPProxyDialer struct {
	profile  *Profile
	proxyURL *url.URL
}

// SOCKS5ProxyDialer creates TLS connections through SOCKS5 proxies with custom fingerprints.
// It uses golang.org/x/net/proxy to establish the SOCKS5 tunnel.
type SOCKS5ProxyDialer struct {
	profile  *Profile
	proxyURL *url.URL
}

// Default TLS fingerprint values captured from Claude Code v2.1.80 (Bun runtime + BoringSSL)
// Captured by intercepting a live ClientHello from Claude Code connecting to a local TLS server.
// Extension order: SNI(0x0000) → ECH(0xfe0d) → EMS(0x0017) → renegotiation(0xff01) → supported_groups(0x000a) →
//
//	ec_point_formats(0x000b) → session_ticket(0x0023) → alpn(0x0010) → status_request(0x0005) →
//	signature_algorithms(0x000d) → sct(0x0012) → key_share(0x0033) → psk_modes(0x002d) →
//	supported_versions(0x002b) → padding(0x0015)
var (
	// defaultCipherSuites contains the latest captured npm/Node TLS cipher suite order (52 suites).
	defaultCipherSuites = []uint16{
		4866, 4867, 4865, 49199, 49195, 49200, 49196, 158, 49191, 103,
		49192, 107, 163, 159, 52393, 52392, 52394, 49325, 49311, 49245,
		49249, 49239, 49235, 162, 49324, 49310, 49244, 49248, 49238, 49234,
		49188, 106, 49187, 64, 49162, 49172, 57, 56, 49161, 49171,
		51, 50, 157, 49309, 49233, 156, 49308, 49232, 61, 60,
		53, 47,
	}

	// defaultCurves contains the captured supported_groups from Claude Code.
	// Byte-equivalent payload (ext 10) from baseline:
	// 001011ec001d0017001e0018001901000101
	defaultCurves = []utls.CurveID{
		utls.CurveID(0x11ec), // X25519MLKEM768 / hybrid group in captured traffic
		utls.X25519,          // 0x001d
		utls.CurveP256,       // 0x0017 (secp256r1)
		utls.CurveID(0x001e), // x448
		utls.CurveP384,       // 0x0018 (secp384r1)
		utls.CurveP521,       // 0x0019 (secp521r1)
		utls.CurveID(0x0100), // ffdhe2048
		utls.CurveID(0x0101), // ffdhe3072
	}

	// defaultPointFormats contains the single point format from Claude Code (Bun/BoringSSL)
	defaultPointFormats = []uint8{
		0, // uncompressed
	}

	// defaultSignatureAlgorithms follows the latest npm/Node captured list.
	defaultSignatureAlgorithms = []utls.SignatureScheme{
		0x0905, 0x0906, 0x0904, 0x0403, 0x0503, 0x0603,
		0x0807, 0x0808, 0x081a, 0x081b, 0x081c, 0x0809,
		0x080a, 0x080b, 0x0804, 0x0805, 0x0806, 0x0401,
		0x0501, 0x0601, 0x0303, 0x0301, 0x0302, 0x0402,
		0x0502, 0x0602,
	}

	// fixedECHData is a deterministic payload used when TLS mode is fixed.
	// This keeps a quick rollback path to one stable 65037 extension payload.
	fixedECHData = mustDecodeBase64("AAABAAE6ACDMQnkkwg7kYVnwrnoD+fs3aXbPCPMI1VzVnxi4RJKMfQCw7TY5/QqUfp+uEmZEauSbD3kUXkCUcM+VSbUkf6Ni7KLIex24vPJaJvFIwnuXOoxKtYNZGwNZku4oLffKmdIuRARCg5gzsAWQZIym+ePe5PqbE5D+LGGx0iSCjVxPXiQR2YuI53ptlWVE95EHZ3xm6a5FYAsOxW5wnUTbprSw4v2e1wZZOFi6cfs1JvCwWJKEhuZHJoUd8Gw13KHTmyAHmYkMXua8/NUOn+G7inEVtXA=")
	// observedECHTemplates keeps known-good ECH(65037) payloads captured from real Claude Code traffic.
	// Using these templates avoids malformed random payloads that can trigger
	// "remote error: tls: error decoding message" during handshake.
	observedECHTemplates = map[int][]byte{
		186: mustDecodeBase64("AAABAAGvACCNpc+T2+xzEWU1kwul1LatFU6is+zNQGRzyg8rYN8xCQCQzYUPbm+zshaySOAk657is70oEI3DlAo0ZQivzM52iL/kzj1Mf8ul1JpZE5FNG05QhZqlD6SMlE7zJ83S2svLzMn1BEmU4wY+9Ls5r9GCZazjPC6sNMU41S3/k67rqWxGC4k9hz+pSixTTozEgpDS8rCxx8g0i1+oucxF3BoNrY3+yMNGMaRmOpvgvQAW2IJQ"),
		218: mustDecodeBase64("AAABAAE6ACDMQnkkwg7kYVnwrnoD+fs3aXbPCPMI1VzVnxi4RJKMfQCw7TY5/QqUfp+uEmZEauSbD3kUXkCUcM+VSbUkf6Ni7KLIex24vPJaJvFIwnuXOoxKtYNZGwNZku4oLffKmdIuRARCg5gzsAWQZIym+ePe5PqbE5D+LGGx0iSCjVxPXiQR2YuI53ptlWVE95EHZ3xm6a5FYAsOxW5wnUTbprSw4v2e1wZZOFi6cfs1JvCwWJKEhuZHJoUd8Gw13KHTmyAHmYkMXua8/NUOn+G7inEVtXA="),
		250: mustDecodeBase64("AAABAAECACDxIu1If0e37bXx/XrCr6wJM9lxMjGIyKd8DmFx975XRQDQQPtWcYEu1Rgkt9gsqev9lL8+Wvyv1nYWsB/o2UyJkFQVUUl7sI/nJmH0nIgifS5s2K1ddbVnRqwXA+G8QQSWzh7MSIwn7XNDESpEN8RhCQlFqSGZzDWdk+eQ9J14PLARcNsDmdZcNpiO03XDApSs9C4o1CIAcBYH4BwBi968NPdKDjDMw1lchPXJ+0dXCerlyi0w9Xofx5WxDSdq6w5bBcIvDlem8EVc3CXjALofFt7zvaKPPgfGOc3x43hAQoDKrfl8GIRpOkK757JojbdP+w=="),
		282: mustDecodeBase64("AAABAAEqACAj8mmxflJFnBK1JCND1B4LqXE54CctLPno4BxVU7T9IwDwSd3k4QnwU4rzxL9mieALyDkfEUoglCO4kfvBWXb0FtxtyW2g92nNNP1xBMRDFvPeAGM9TFAYEE2wkunOnz9OL3HouyaiR5L7MXSbrQRgi5Q6d63PJ5THCJflKznCrHrOkbB4rVfYQ8rMPlBd3DQ8aySNHuyvIcufJLp/3Eb2vVbwCa10erJsipTETi2780140/4j7u+HyFmMcW5A6FPGLP2w3JVZ62wQ8Uf/FqZFiAJGGdroKOTyNme+XvF7zeSokVnh7BGxhaAA0FP/A87tPJJINOZh94OZimnufK4Gofz3ZEwZg3HWdKrOnscjjcZY"),
	}
)

type tlsShape struct {
	echLen     int
	paddingLen int // 0 means no ext 21
	weight     int
}

type tlsShapeSchedulerState struct {
	windowSize int
	history    []int // selected shape index history
	pos        int
	counts     []int
}

type TLSShapeScheduler struct {
	mu     sync.Mutex
	states map[string]*tlsShapeSchedulerState
}

var globalTLSShapeScheduler = NewTLSShapeScheduler()

func NewTLSShapeScheduler() *TLSShapeScheduler {
	return &TLSShapeScheduler{states: make(map[string]*tlsShapeSchedulerState)}
}

var (
	defaultTLSShapeWeights = TLSShapeWeights{
		ECH186Padding41: 20,
		ECH218Padding9:  22,
		ECH250Padding0:  23,
		ECH282Padding0:  35,
	}
)

const (
	// npmPSKSessionTTL follows the observed effective timeout from live ticket capture:
	// "Timeout: 7200 (sec)".
	npmPSKSessionTTL = 2 * time.Hour
	// npmPSKCacheCapacity is enough for multi-account/account-proxy isolated caches.
	npmPSKCacheCapacity = 256
)

const tlsFPInsecureSkipVerifyEnv = "SUB2API_TLSFP_INSECURE_SKIP_VERIFY"

func shouldSkipVerifyForTest() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(tlsFPInsecureSkipVerifyEnv)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

type expiringClientSessionCache struct {
	base    utls.ClientSessionCache
	ttl     time.Duration
	mu      sync.RWMutex
	expires map[string]time.Time
}

func newExpiringClientSessionCache(capacity int, ttl time.Duration) utls.ClientSessionCache {
	if capacity <= 0 {
		capacity = npmPSKCacheCapacity
	}
	if ttl <= 0 {
		ttl = npmPSKSessionTTL
	}
	return &expiringClientSessionCache{
		base:    utls.NewLRUClientSessionCache(capacity),
		ttl:     ttl,
		expires: make(map[string]time.Time, capacity),
	}
}

func (c *expiringClientSessionCache) Get(sessionKey string) (*utls.ClientSessionState, bool) {
	now := time.Now()

	c.mu.RLock()
	exp, ok := c.expires[sessionKey]
	c.mu.RUnlock()
	if ok && now.After(exp) {
		c.mu.Lock()
		delete(c.expires, sessionKey)
		c.mu.Unlock()
		c.base.Put(sessionKey, nil)
		return nil, false
	}
	return c.base.Get(sessionKey)
}

func (c *expiringClientSessionCache) Put(sessionKey string, cs *utls.ClientSessionState) {
	c.base.Put(sessionKey, cs)
	c.mu.Lock()
	defer c.mu.Unlock()
	if cs == nil {
		delete(c.expires, sessionKey)
		return
	}
	c.expires[sessionKey] = time.Now().Add(c.ttl)
}

type pskSessionCacheRegistry struct {
	mu     sync.Mutex
	caches map[string]utls.ClientSessionCache
}

var globalPSKSessionCacheRegistry = &pskSessionCacheRegistry{
	caches: make(map[string]utls.ClientSessionCache),
}

func (r *pskSessionCacheRegistry) getOrCreate(key string) utls.ClientSessionCache {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cache, ok := r.caches[key]; ok {
		return cache
	}
	cache := newExpiringClientSessionCache(npmPSKCacheCapacity, npmPSKSessionTTL)
	r.caches[key] = cache
	return cache
}

func shouldEnablePreSharedKey(profile *Profile) bool {
	if profile == nil {
		return false
	}
	switch strings.TrimSpace(profile.ECHScopeKey) {
	case "messages", "messages_count_tokens":
		return true
	default:
		return false
	}
}

func preSharedKeySessionCache(profile *Profile) utls.ClientSessionCache {
	if !shouldEnablePreSharedKey(profile) {
		return nil
	}
	cacheKey := "npm:messages"
	if profile != nil {
		if v := strings.TrimSpace(profile.ClientCacheKey); v != "" {
			cacheKey = v
		} else if v := strings.TrimSpace(profile.ECHScopeKey); v != "" {
			cacheKey = "npm:" + v
		}
	}
	return globalPSKSessionCacheRegistry.getOrCreate(cacheKey)
}

func mustDecodeBase64(v string) []byte {
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		panic(err)
	}
	return b
}

// NewDialer creates a new TLS fingerprint dialer.
// baseDialer is used for TCP connection establishment (supports proxy scenarios).
// If baseDialer is nil, direct TCP dial is used.
func NewDialer(profile *Profile, baseDialer func(ctx context.Context, network, addr string) (net.Conn, error)) *Dialer {
	if baseDialer == nil {
		baseDialer = (&net.Dialer{}).DialContext
	}
	return &Dialer{profile: profile, baseDialer: baseDialer}
}

// NewHTTPProxyDialer creates a new TLS fingerprint dialer that works through HTTP/HTTPS proxies.
// It establishes a CONNECT tunnel before performing TLS handshake with custom fingerprint.
func NewHTTPProxyDialer(profile *Profile, proxyURL *url.URL) *HTTPProxyDialer {
	return &HTTPProxyDialer{profile: profile, proxyURL: proxyURL}
}

// NewSOCKS5ProxyDialer creates a new TLS fingerprint dialer that works through SOCKS5 proxies.
// It establishes a SOCKS5 tunnel before performing TLS handshake with custom fingerprint.
func NewSOCKS5ProxyDialer(profile *Profile, proxyURL *url.URL) *SOCKS5ProxyDialer {
	return &SOCKS5ProxyDialer{profile: profile, proxyURL: proxyURL}
}

// DialTLSContext establishes a TLS connection through SOCKS5 proxy with the configured fingerprint.
// Flow: SOCKS5 CONNECT to target -> TLS handshake with utls on the tunnel
func (d *SOCKS5ProxyDialer) DialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	slog.Debug("tls_fingerprint_socks5_connecting", "proxy", d.proxyURL.Host, "target", addr)

	// Step 1: Create SOCKS5 dialer
	var auth *proxy.Auth
	if d.proxyURL.User != nil {
		username := d.proxyURL.User.Username()
		password, _ := d.proxyURL.User.Password()
		auth = &proxy.Auth{
			User:     username,
			Password: password,
		}
	}

	// Determine proxy address
	proxyAddr := d.proxyURL.Host
	if d.proxyURL.Port() == "" {
		proxyAddr = net.JoinHostPort(d.proxyURL.Hostname(), "1080") // Default SOCKS5 port
	}

	socksDialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		slog.Debug("tls_fingerprint_socks5_dialer_failed", "error", err)
		return nil, fmt.Errorf("create SOCKS5 dialer: %w", err)
	}

	// Step 2: Establish SOCKS5 tunnel to target
	slog.Debug("tls_fingerprint_socks5_establishing_tunnel", "target", addr)
	conn, err := socksDialer.Dial("tcp", addr)
	if err != nil {
		slog.Debug("tls_fingerprint_socks5_connect_failed", "error", err)
		return nil, fmt.Errorf("SOCKS5 connect: %w", err)
	}
	slog.Debug("tls_fingerprint_socks5_tunnel_established")

	// Step 3: Perform TLS handshake on the tunnel with utls fingerprint
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	slog.Debug("tls_fingerprint_socks5_starting_handshake", "host", host)

	// Build ClientHello specification from profile (Node.js/Claude CLI fingerprint)
	spec := buildClientHelloSpecFromProfile(d.profile)
	slog.Debug("tls_fingerprint_socks5_clienthello_spec",
		"cipher_suites", len(spec.CipherSuites),
		"extensions", len(spec.Extensions),
		"compression_methods", spec.CompressionMethods,
		"tls_vers_max", spec.TLSVersMax,
		"tls_vers_min", spec.TLSVersMin)

	if d.profile != nil {
		slog.Debug("tls_fingerprint_socks5_using_profile", "name", d.profile.Name, "grease", d.profile.EnableGREASE)
	}

	// Create uTLS connection on the tunnel
	tlsCfg := &utls.Config{
		ServerName:                         host,
		OmitEmptyPsk:                      true,
		PreferSkipResumptionOnNilExtension: true,
	}
	if shouldSkipVerifyForTest() {
		tlsCfg.InsecureSkipVerify = true
	}
	if cache := preSharedKeySessionCache(d.profile); cache != nil {
		tlsCfg.ClientSessionCache = cache
	}
	tlsConn := utls.UClient(conn, tlsCfg, utls.HelloCustom)

	if err := tlsConn.ApplyPreset(spec); err != nil {
		slog.Debug("tls_fingerprint_socks5_apply_preset_failed", "error", err)
		_ = conn.Close()
		return nil, fmt.Errorf("apply TLS preset: %w", err)
	}

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		slog.Debug("tls_fingerprint_socks5_handshake_failed", "error", err)
		_ = conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	state := tlsConn.ConnectionState()
	slog.Debug("tls_fingerprint_socks5_handshake_success",
		"version", state.Version,
		"cipher_suite", state.CipherSuite,
		"alpn", state.NegotiatedProtocol)

	return tlsConn, nil
}

// DialTLSContext establishes a TLS connection through HTTP proxy with the configured fingerprint.
// Flow: TCP connect to proxy -> CONNECT tunnel -> TLS handshake with utls
func (d *HTTPProxyDialer) DialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	slog.Debug("tls_fingerprint_http_proxy_connecting", "proxy", d.proxyURL.Host, "target", addr)

	// Step 1: TCP connect to proxy server
	var proxyAddr string
	if d.proxyURL.Port() != "" {
		proxyAddr = d.proxyURL.Host
	} else {
		// Default ports
		if d.proxyURL.Scheme == "https" {
			proxyAddr = net.JoinHostPort(d.proxyURL.Hostname(), "443")
		} else {
			proxyAddr = net.JoinHostPort(d.proxyURL.Hostname(), "80")
		}
	}

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		slog.Debug("tls_fingerprint_http_proxy_connect_failed", "error", err)
		return nil, fmt.Errorf("connect to proxy: %w", err)
	}
	slog.Debug("tls_fingerprint_http_proxy_connected", "proxy_addr", proxyAddr)

	// Step 2: Send CONNECT request to establish tunnel
	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}

	// Add proxy authentication if present
	if d.proxyURL.User != nil {
		username := d.proxyURL.User.Username()
		password, _ := d.proxyURL.User.Password()
		auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		req.Header.Set("Proxy-Authorization", "Basic "+auth)
	}

	slog.Debug("tls_fingerprint_http_proxy_sending_connect", "target", addr)
	if err := req.Write(conn); err != nil {
		_ = conn.Close()
		slog.Debug("tls_fingerprint_http_proxy_write_failed", "error", err)
		return nil, fmt.Errorf("write CONNECT request: %w", err)
	}

	// Step 3: Read CONNECT response
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		_ = conn.Close()
		slog.Debug("tls_fingerprint_http_proxy_read_response_failed", "error", err)
		return nil, fmt.Errorf("read CONNECT response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		slog.Debug("tls_fingerprint_http_proxy_connect_failed_status", "status_code", resp.StatusCode, "status", resp.Status)
		return nil, fmt.Errorf("proxy CONNECT failed: %s", resp.Status)
	}
	slog.Debug("tls_fingerprint_http_proxy_tunnel_established")

	// Step 4: Perform TLS handshake on the tunnel with utls fingerprint
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	slog.Debug("tls_fingerprint_http_proxy_starting_handshake", "host", host)

	// Build ClientHello specification (reuse the shared method)
	spec := buildClientHelloSpecFromProfile(d.profile)
	slog.Debug("tls_fingerprint_http_proxy_clienthello_spec",
		"cipher_suites", len(spec.CipherSuites),
		"extensions", len(spec.Extensions))

	if d.profile != nil {
		slog.Debug("tls_fingerprint_http_proxy_using_profile", "name", d.profile.Name, "grease", d.profile.EnableGREASE)
	}

	// Create uTLS connection on the tunnel
	// Note: TLS 1.3 cipher suites are handled automatically by utls when TLS 1.3 is in SupportedVersions
	tlsCfg := &utls.Config{
		ServerName:                         host,
		OmitEmptyPsk:                      true,
		PreferSkipResumptionOnNilExtension: true,
	}
	if shouldSkipVerifyForTest() {
		tlsCfg.InsecureSkipVerify = true
	}
	if cache := preSharedKeySessionCache(d.profile); cache != nil {
		tlsCfg.ClientSessionCache = cache
	}
	tlsConn := utls.UClient(conn, tlsCfg, utls.HelloCustom)

	if err := tlsConn.ApplyPreset(spec); err != nil {
		slog.Debug("tls_fingerprint_http_proxy_apply_preset_failed", "error", err)
		_ = conn.Close()
		return nil, fmt.Errorf("apply TLS preset: %w", err)
	}

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		slog.Debug("tls_fingerprint_http_proxy_handshake_failed", "error", err)
		_ = conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	state := tlsConn.ConnectionState()
	slog.Debug("tls_fingerprint_http_proxy_handshake_success",
		"version", state.Version,
		"cipher_suite", state.CipherSuite,
		"alpn", state.NegotiatedProtocol)

	return tlsConn, nil
}

// DialTLSContext establishes a TLS connection with the configured fingerprint.
// This method is designed to be used as http.Transport.DialTLSContext.
func (d *Dialer) DialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// Establish TCP connection using base dialer (supports proxy)
	slog.Debug("tls_fingerprint_dialing_tcp", "addr", addr)
	conn, err := d.baseDialer(ctx, network, addr)
	if err != nil {
		slog.Debug("tls_fingerprint_tcp_dial_failed", "error", err)
		return nil, err
	}
	slog.Debug("tls_fingerprint_tcp_connected", "addr", addr)

	// Extract hostname for SNI
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	slog.Debug("tls_fingerprint_sni_hostname", "host", host)

	// Build ClientHello specification
	spec := d.buildClientHelloSpec()
	slog.Debug("tls_fingerprint_clienthello_spec",
		"cipher_suites", len(spec.CipherSuites),
		"extensions", len(spec.Extensions))

	// Log profile info
	if d.profile != nil {
		slog.Debug("tls_fingerprint_using_profile", "name", d.profile.Name, "grease", d.profile.EnableGREASE)
	} else {
		slog.Debug("tls_fingerprint_using_default_profile")
	}

	// Create uTLS connection
	// Note: TLS 1.3 cipher suites are handled automatically by utls when TLS 1.3 is in SupportedVersions
	tlsCfg := &utls.Config{
		ServerName:                         host,
		OmitEmptyPsk:                      true,
		PreferSkipResumptionOnNilExtension: true,
	}
	if shouldSkipVerifyForTest() {
		tlsCfg.InsecureSkipVerify = true
	}
	if cache := preSharedKeySessionCache(d.profile); cache != nil {
		tlsCfg.ClientSessionCache = cache
	}
	tlsConn := utls.UClient(conn, tlsCfg, utls.HelloCustom)

	// Apply fingerprint
	if err := tlsConn.ApplyPreset(spec); err != nil {
		slog.Debug("tls_fingerprint_apply_preset_failed", "error", err)
		_ = conn.Close()
		return nil, err
	}
	slog.Debug("tls_fingerprint_preset_applied")

	// Perform TLS handshake
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		slog.Debug("tls_fingerprint_handshake_failed",
			"error", err,
			"local_addr", conn.LocalAddr(),
			"remote_addr", conn.RemoteAddr())
		_ = conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Log successful handshake details
	state := tlsConn.ConnectionState()
	slog.Debug("tls_fingerprint_handshake_success",
		"version", state.Version,
		"cipher_suite", state.CipherSuite,
		"alpn", state.NegotiatedProtocol)

	return tlsConn, nil
}

// buildClientHelloSpec constructs the ClientHello specification based on the profile.
func (d *Dialer) buildClientHelloSpec() *utls.ClientHelloSpec {
	return buildClientHelloSpecFromProfile(d.profile)
}

// toUTLSCurves converts uint16 slice to utls.CurveID slice.
func toUTLSCurves(curves []uint16) []utls.CurveID {
	result := make([]utls.CurveID, len(curves))
	for i, c := range curves {
		result[i] = utls.CurveID(c)
	}
	return result
}

// buildClientHelloSpecFromProfile constructs ClientHelloSpec from a Profile.
// This is a standalone function that can be used by both Dialer and HTTPProxyDialer.
func buildClientHelloSpecFromProfile(profile *Profile) *utls.ClientHelloSpec {
	// Get cipher suites
	var cipherSuites []uint16
	if profile != nil && len(profile.CipherSuites) > 0 {
		cipherSuites = profile.CipherSuites
	} else {
		cipherSuites = defaultCipherSuites
	}

	// Get curves
	var curves []utls.CurveID
	if profile != nil && len(profile.Curves) > 0 {
		curves = toUTLSCurves(profile.Curves)
	} else {
		curves = defaultCurves
	}

	// Get point formats
	var pointFormats []uint8
	if profile != nil && len(profile.PointFormats) > 0 {
		pointFormats = profile.PointFormats
	} else {
		pointFormats = defaultPointFormats
	}

	scope := ""
	if profile != nil {
		scope = strings.TrimSpace(profile.ECHScopeKey)
	}
	includeALPN := shouldIncludeALPN(scope)
	// npm-like extension order from latest capture:
	// 65281,0,11,10,35,(16),22,23,13,43,45,51,(41 when PSK available)
	extensions := make([]utls.TLSExtension, 0, 12)
	extensions = append(extensions,
		&utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient}, // 65281
		&utls.SNIExtension{}, // 0
		&utls.SupportedPointsExtension{SupportedPoints: pointFormats}, // 11
		&utls.SupportedCurvesExtension{Curves: curves},                // 10
		&utls.SessionTicketExtension{},                                // 35
	)
	if includeALPN {
		extensions = append(extensions, &utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}}) // 16
	}
	extensions = append(extensions,
		&utls.GenericExtension{Id: 22, Data: []byte{}},                                               // 22 encrypt_then_mac
		&utls.ExtendedMasterSecretExtension{},                                                        // 23
		&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: defaultSignatureAlgorithms}, // 13
		&utls.SupportedVersionsExtension{Versions: []uint16{utls.VersionTLS13, utls.VersionTLS12}},   // 43
		&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},                          // 45
		// Captured shape uses two shares: X25519MLKEM768 + X25519.
		&utls.KeyShareExtension{KeyShares: []utls.KeyShare{
			{Group: utls.CurveID(0x11ec)}, // X25519MLKEM768
			{Group: utls.X25519},
		}}, // 51
	)
	if shouldEnablePreSharedKey(profile) {
		extensions = append(extensions, &utls.UtlsPreSharedKeyExtension{}) // 41
	}

	return &utls.ClientHelloSpec{
		CipherSuites:       cipherSuites,
		CompressionMethods: []uint8{0}, // null compression only (standard)
		Extensions:         extensions,
		TLSVersMax:         utls.VersionTLS13,
		TLSVersMin:         utls.VersionTLS10,
	}
}

func shouldIncludeALPN(scope string) bool {
	switch scope {
	case "mcp_servers", "oauth_usage":
		return false
	default:
		return true
	}
}

func buildECHOuterExtension(mode string, shape tlsShape, payloadMode string, profile *Profile) utls.TLSExtension {
	mode = normalizeMode(mode)
	switch mode {
	case "fixed":
		return &utls.GenericExtension{Id: 65037, Data: fixedECHData}
	default:
		return &utls.GenericExtension{Id: 65037, Data: buildECHPayload(shape.echLen, payloadMode, profile)}
	}
}

func shouldIncludePadding(mode string, shape tlsShape) bool {
	mode = normalizeMode(mode)
	switch mode {
	case "fixed":
		return true
	default:
		return shape.paddingLen > 0
	}
}

func buildPaddingExtension(mode string, shape tlsShape) utls.TLSExtension {
	mode = normalizeMode(mode)
	if mode == "fixed" || shape.paddingLen <= 0 {
		return &utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle}
	}
	fixedPadding := shape.paddingLen
	return &utls.UtlsPaddingExtension{
		GetPaddingLen: func(clientHelloLen int) (paddingLen int, willPad bool) {
			return fixedPadding, true
		},
	}
}

func selectTLSShape(profile *Profile, mode string) tlsShape {
	mode = normalizeMode(mode)
	shapeMode := "observed_v1"
	windowSize := 64
	if profile != nil {
		shapeMode = normalizeShapeMode(profile.ShapeMode)
		if profile.ShapeWindowSize > 0 {
			windowSize = profile.ShapeWindowSize
		}
	}

	weights := getShapeWeights(profile)
	shapes := buildShapePool(mode, weights)
	if len(shapes) == 0 {
		return tlsShape{echLen: 250, paddingLen: 0}
	}

	switch mode {
	case "fixed":
		return tlsShape{echLen: len(fixedECHData), paddingLen: 0}
	default:
		if shapeMode == "random" {
			return chooseWeightedShape(shapes)
		}
		return globalTLSShapeScheduler.Select(profileSchedulerKey(profile, mode, shapeMode, windowSize, weights), windowSize, shapes)
	}
}

func (s *TLSShapeScheduler) Select(key string, windowSize int, shapes []tlsShape) tlsShape {
	if len(shapes) == 0 {
		return tlsShape{echLen: 250, paddingLen: 0}
	}
	if windowSize <= 0 {
		return chooseWeightedShape(shapes)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.states[key]
	if state == nil || state.windowSize != windowSize || len(state.counts) != len(shapes) {
		state = &tlsShapeSchedulerState{
			windowSize: windowSize,
			history:    make([]int, 0, windowSize),
			counts:     make([]int, len(shapes)),
		}
		s.states[key] = state
	}

	nextN := len(state.history) + 1
	if nextN > windowSize {
		nextN = windowSize
	}

	totalWeight := 0
	for _, sh := range shapes {
		if sh.weight > 0 {
			totalWeight += sh.weight
		}
	}
	if totalWeight <= 0 {
		return chooseWeightedShape(shapes)
	}

	bestIdx := 0
	bestDeficit := -1e9
	bestWeight := -1
	for i, sh := range shapes {
		if sh.weight <= 0 {
			continue
		}
		target := float64(nextN) * float64(sh.weight) / float64(totalWeight)
		current := float64(state.counts[i])
		deficit := target - current
		if deficit > bestDeficit || (deficit == bestDeficit && sh.weight > bestWeight) {
			bestDeficit = deficit
			bestWeight = sh.weight
			bestIdx = i
		}
	}

	if len(state.history) < windowSize {
		state.history = append(state.history, bestIdx)
	} else {
		evict := state.history[state.pos]
		if evict >= 0 && evict < len(state.counts) {
			state.counts[evict]--
		}
		state.history[state.pos] = bestIdx
		state.pos = (state.pos + 1) % windowSize
	}
	state.counts[bestIdx]++
	return shapes[bestIdx]
}

func chooseWeightedShape(shapes []tlsShape) tlsShape {
	if len(shapes) == 0 {
		return tlsShape{echLen: 250, paddingLen: 0}
	}
	total := 0
	for _, s := range shapes {
		if s.weight > 0 {
			total += s.weight
		}
	}
	if total <= 0 {
		return shapes[0]
	}
	n := cryptoRandInt(total)
	acc := 0
	for _, s := range shapes {
		if s.weight <= 0 {
			continue
		}
		acc += s.weight
		if n < acc {
			return s
		}
	}
	return shapes[len(shapes)-1]
}

func cryptoRandInt(max int) int {
	if max <= 1 {
		return 0
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Safe deterministic fallback for rare entropy failures.
		return 0
	}
	return int(binary.BigEndian.Uint64(b) % uint64(max))
}

func randomBytes(n int) []byte {
	if n <= 0 {
		return nil
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return make([]byte, n)
	}
	return b
}

func buildECHPayload(length int, mode string, profile *Profile) []byte {
	mode = normalizeECHPayloadMode(mode)
	switch mode {
	case "oauth_outer":
		return buildOAuthOuterECHPayload(length, profile)
	case "random":
		return randomBytes(length)
	default:
		return templatedECHPayload(length)
	}
}

type parsedECHConfig struct {
	configID byte
	kemID    uint16
	kdfID    uint16
	aeadID   uint16
}

func parseECHConfigList(cfgList []byte) (parsedECHConfig, bool) {
	if len(cfgList) < 2 {
		return parsedECHConfig{}, false
	}
	listLen := int(binary.BigEndian.Uint16(cfgList[:2]))
	if listLen <= 0 || listLen+2 > len(cfgList) {
		return parsedECHConfig{}, false
	}
	p := cfgList[2 : 2+listLen]
	off := 0
	for off+4 <= len(p) {
		ver := binary.BigEndian.Uint16(p[off : off+2])
		off += 2
		cfgLen := int(binary.BigEndian.Uint16(p[off : off+2]))
		off += 2
		if cfgLen <= 0 || off+cfgLen > len(p) {
			return parsedECHConfig{}, false
		}
		cfg := p[off : off+cfgLen]
		off += cfgLen
		if ver != 0xfe0d || len(cfg) < 8 {
			continue
		}
		cOff := 0
		configID := cfg[cOff]
		cOff++
		kemID := binary.BigEndian.Uint16(cfg[cOff : cOff+2])
		cOff += 2
		pubKeyLen := int(binary.BigEndian.Uint16(cfg[cOff : cOff+2]))
		cOff += 2
		if pubKeyLen < 0 || cOff+pubKeyLen > len(cfg) {
			return parsedECHConfig{}, false
		}
		cOff += pubKeyLen
		if cOff+2 > len(cfg) {
			return parsedECHConfig{}, false
		}
		suiteLen := int(binary.BigEndian.Uint16(cfg[cOff : cOff+2]))
		cOff += 2
		if suiteLen < 4 || cOff+suiteLen > len(cfg) {
			return parsedECHConfig{}, false
		}
		kdfID := binary.BigEndian.Uint16(cfg[cOff : cOff+2])
		aeadID := binary.BigEndian.Uint16(cfg[cOff+2 : cOff+4])
		return parsedECHConfig{
			configID: configID,
			kemID:    kemID,
			kdfID:    kdfID,
			aeadID:   aeadID,
		}, true
	}
	return parsedECHConfig{}, false
}

func kemEncLen(kemID uint16) int {
	switch kemID {
	case 0x0020: // X25519
		return 32
	case 0x0010: // P-256
		return 65
	case 0x0011: // P-384
		return 97
	case 0x0012: // P-521
		return 133
	default:
		return 32
	}
}

func randomByte() byte {
	b := randomBytes(1)
	if len(b) == 0 {
		return 0
	}
	return b[0]
}

// buildOAuthOuterECHPayload generates a GREASE-like ECH outer extension payload
// matching the observed structure from Claude Code OAuth traffic:
// [type(1), kdf(2), aead(2), config_id(1), enc_len(2), enc, payload_len(2), payload]
func buildOAuthOuterECHPayload(length int, profile *Profile) []byte {
	if length <= 0 {
		return nil
	}
	cfg, ok := parseECHConfigList(nil)
	if profile != nil {
		cfg, ok = parseECHConfigList(profile.ECHConfigList)
	}
	if !ok {
		return templatedECHPayload(length)
	}
	encLen := kemEncLen(cfg.kemID)
	overhead := 1 + 2 + 2 + 1 + 2 + encLen + 2
	if length < overhead {
		return templatedECHPayload(length)
	}
	payloadLen := length - overhead
	out := make([]byte, length)
	out[0] = 0x00 // outer_client_hello
	binary.BigEndian.PutUint16(out[1:3], cfg.kdfID)
	binary.BigEndian.PutUint16(out[3:5], cfg.aeadID)
	// Claude Code observed traffic shows varying config_id in GREASE-like 65037.
	// Keep endpoint-awareness via scope key while preserving per-connection variation.
	configID := cfg.configID
	if profile != nil && profile.ECHScopeKey != "" {
		var seed byte
		for i := 0; i < len(profile.ECHScopeKey); i++ {
			seed ^= profile.ECHScopeKey[i]
		}
		configID = seed ^ randomByte()
	} else {
		configID = randomByte()
	}
	out[5] = configID
	binary.BigEndian.PutUint16(out[6:8], uint16(encLen))
	enc := randomBytes(encLen)
	copy(out[8:8+encLen], enc)
	binary.BigEndian.PutUint16(out[8+encLen:10+encLen], uint16(payloadLen))
	copy(out[10+encLen:], randomBytes(payloadLen))
	return out
}

func templatedECHPayload(length int) []byte {
	if length <= 0 {
		return nil
	}
	if tpl, ok := observedECHTemplates[length]; ok && len(tpl) == length {
		out := make([]byte, length)
		copy(out, tpl)
		return out
	}
	// Safe fallback for unknown lengths: use deterministic known-good payload and resize.
	out := make([]byte, length)
	copy(out, fixedECHData)
	if len(fixedECHData) < length {
		copy(out[len(fixedECHData):], randomBytes(length-len(fixedECHData)))
	}
	return out
}

func normalizeShapeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "npm" {
		return mode
	}
	return "npm"
}

func normalizeECHPayloadMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "npm" {
		return mode
	}
	return "npm"
}

func getShapeWeights(profile *Profile) TLSShapeWeights {
	if profile == nil {
		return defaultTLSShapeWeights
	}
	w := profile.ShapeWeights
	if w.ECH186Padding41+w.ECH218Padding9+w.ECH250Padding0+w.ECH282Padding0 <= 0 {
		return defaultTLSShapeWeights
	}
	return w
}

func buildShapePool(mode string, weights TLSShapeWeights) []tlsShape {
	mode = normalizeMode(mode)
	mixed := []tlsShape{
		{echLen: 186, paddingLen: 41, weight: weights.ECH186Padding41},
		{echLen: 218, paddingLen: 9, weight: weights.ECH218Padding9},
		{echLen: 250, paddingLen: 0, weight: weights.ECH250Padding0},
		{echLen: 282, paddingLen: 0, weight: weights.ECH282Padding0},
	}
	if mode == "dynamic" {
		return []tlsShape{
			{echLen: 186, paddingLen: 41, weight: weights.ECH186Padding41},
			{echLen: 218, paddingLen: 9, weight: weights.ECH218Padding9},
		}
	}
	return mixed
}

func profileSchedulerKey(profile *Profile, mode, shapeMode string, windowSize int, weights TLSShapeWeights) string {
	name := ""
	scope := ""
	if profile != nil {
		name = profile.Name
		scope = profile.ECHScopeKey
	}
	return fmt.Sprintf("%s|%s|%s|scope=%s|w=%d|%d,%d,%d,%d",
		name, mode, shapeMode, scope, windowSize,
		weights.ECH186Padding41, weights.ECH218Padding9, weights.ECH250Padding0, weights.ECH282Padding0,
	)
}

func normalizeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "npm" {
		return mode
	}
	return "npm"
}
