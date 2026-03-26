package tlsfingerprint

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"sort"
	"strings"
	"time"
)

// RawHTTP1RoundTripper writes an HTTP/1.1 request manually so header order is deterministic.
// It is intended for fingerprint-sensitive upstreams.
type RawHTTP1RoundTripper struct {
	DialTLSContext        func(ctx context.Context, network, addr string) (net.Conn, error)
	ResponseHeaderTimeout time.Duration
}

func NewRawHTTP1RoundTripper(
	dialTLSContext func(ctx context.Context, network, addr string) (net.Conn, error),
	responseHeaderTimeout time.Duration,
) *RawHTTP1RoundTripper {
	return &RawHTTP1RoundTripper{
		DialTLSContext:        dialTLSContext,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}
}

func (rt *RawHTTP1RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("nil request")
	}
	if rt == nil || rt.DialTLSContext == nil {
		return nil, fmt.Errorf("raw round tripper is not configured")
	}
	if !strings.EqualFold(strings.TrimSpace(req.URL.Scheme), "https") {
		return nil, fmt.Errorf("raw round tripper only supports https")
	}

	addr := req.URL.Host
	if !strings.Contains(addr, ":") {
		addr += ":443"
	}

	conn, err := rt.DialTLSContext(req.Context(), "tcp", addr)
	if err != nil {
		return nil, err
	}

	if rt.ResponseHeaderTimeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(rt.ResponseHeaderTimeout))
	}

	bw := bufio.NewWriter(conn)
	uri := req.URL.RequestURI()
	if uri == "" {
		uri = "/"
	}
	if _, err = fmt.Fprintf(bw, "%s %s HTTP/1.1\r\n", req.Method, uri); err != nil {
		_ = conn.Close()
		return nil, err
	}

	host := req.Host
	if strings.TrimSpace(host) == "" {
		host = req.URL.Host
	}
	if err = writeOrderedHeaders(bw, req, host); err != nil {
		_ = conn.Close()
		return nil, err
	}

	if _, err = bw.WriteString("\r\n"); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if req.Body != nil {
		if _, err = io.Copy(bw, req.Body); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	if err = bw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})

	resp.Body = &rawConnBody{ReadCloser: resp.Body, conn: conn}
	return resp, nil
}

type rawConnBody struct {
	io.ReadCloser
	conn net.Conn
}

func (b *rawConnBody) Close() error {
	err := b.ReadCloser.Close()
	_ = b.conn.Close()
	return err
}

func writeOrderedHeaders(w io.Writer, req *http.Request, host string) error {
	preferred := preferredHeaderOrder(req)

	if req.Body != nil && req.ContentLength >= 0 && len(req.Header.Values("content-length")) == 0 {
		req.Header.Set("Content-Length", fmt.Sprintf("%d", req.ContentLength))
	}

	written := make(map[string]struct{}, len(req.Header)+8)
	writeHost := func() error {
		if strings.TrimSpace(host) == "" {
			return nil
		}
		if _, err := fmt.Fprintf(w, "%s: %s\r\n", wireHeaderKey(req, "host"), host); err != nil {
			return err
		}
		written["host"] = struct{}{}
		return nil
	}
	writeKey := func(lowerKey string) error {
		if lowerKey == "host" {
			return writeHost()
		}
		values := req.Header.Values(lowerKey)
		if len(values) == 0 {
			return nil
		}
		wireKey := wireHeaderKey(req, lowerKey)
		for _, v := range values {
			if _, err := fmt.Fprintf(w, "%s: %s\r\n", wireKey, v); err != nil {
				return err
			}
		}
		written[lowerKey] = struct{}{}
		return nil
	}

	for _, k := range preferred {
		if err := writeKey(k); err != nil {
			return err
		}
	}

	rest := make([]string, 0, len(req.Header)+1)
	seen := make(map[string]struct{}, len(req.Header)+1)
	for k := range req.Header {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "" {
			continue
		}
		if _, ok := written[lk]; ok {
			continue
		}
		if _, ok := seen[lk]; ok {
			continue
		}
		seen[lk] = struct{}{}
		rest = append(rest, lk)
	}
	if strings.TrimSpace(host) != "" {
		if _, ok := written["host"]; !ok {
			rest = append(rest, "host")
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		if err := writeKey(k); err != nil {
			return err
		}
	}
	return nil
}

func wireHeaderKey(req *http.Request, lowerKey string) string {
	if req != nil && req.URL != nil && strings.EqualFold(strings.TrimSpace(req.URL.Hostname()), "api.anthropic.com") {
		path := strings.TrimSpace(req.URL.Path)
		switch {
		case req.Method == http.MethodPost && strings.HasPrefix(path, "/v1/messages/count_tokens"):
			return anthropicMessagesWireHeaderKey(lowerKey)
		case req.Method == http.MethodPost && strings.HasPrefix(path, "/v1/messages"):
			return anthropicMessagesWireHeaderKey(lowerKey)
		case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/mcp_servers"):
			return anthropicMCPWireHeaderKey(lowerKey)
		case req.Method == http.MethodGet && strings.HasPrefix(path, "/api/oauth/usage"):
			return anthropicOAuthUsageWireHeaderKey(lowerKey)
		}
	}
	return textproto.CanonicalMIMEHeaderKey(lowerKey)
}

func anthropicMessagesWireHeaderKey(lowerKey string) string {
	switch lowerKey {
	case "host", "connection", "authorization", "x-app", "content-type", "anthropic-dangerous-direct-browser-access",
		"anthropic-version", "anthropic-beta", "x-client-request-id", "accept-language", "sec-fetch-mode",
		"accept-encoding", "content-length":
		return lowerKey
	case "accept":
		return "Accept"
	case "user-agent":
		return "User-Agent"
	case "x-stainless-retry-count":
		return "X-Stainless-Retry-Count"
	case "x-stainless-timeout":
		return "X-Stainless-Timeout"
	case "x-stainless-lang":
		return "X-Stainless-Lang"
	case "x-stainless-package-version":
		return "X-Stainless-Package-Version"
	case "x-stainless-os":
		return "X-Stainless-OS"
	case "x-stainless-arch":
		return "X-Stainless-Arch"
	case "x-stainless-runtime":
		return "X-Stainless-Runtime"
	case "x-stainless-runtime-version":
		return "X-Stainless-Runtime-Version"
	default:
		return textproto.CanonicalMIMEHeaderKey(lowerKey)
	}
}

func anthropicMCPWireHeaderKey(lowerKey string) string {
	switch lowerKey {
	case "anthropic-beta", "anthropic-version":
		return lowerKey
	case "accept":
		return "Accept"
	case "content-type":
		return "Content-Type"
	case "authorization":
		return "Authorization"
	case "user-agent":
		return "User-Agent"
	case "accept-encoding":
		return "Accept-Encoding"
	case "host":
		return "Host"
	case "connection":
		return "Connection"
	default:
		return textproto.CanonicalMIMEHeaderKey(lowerKey)
	}
}

func anthropicOAuthUsageWireHeaderKey(lowerKey string) string {
	switch lowerKey {
	case "anthropic-beta":
		return lowerKey
	case "accept":
		return "Accept"
	case "content-type":
		return "Content-Type"
	case "user-agent":
		return "User-Agent"
	case "authorization":
		return "Authorization"
	case "accept-encoding":
		return "Accept-Encoding"
	case "host":
		return "Host"
	case "connection":
		return "Connection"
	default:
		return textproto.CanonicalMIMEHeaderKey(lowerKey)
	}
}

func preferredHeaderOrder(req *http.Request) []string {
	if req != nil && req.URL != nil && strings.EqualFold(strings.TrimSpace(req.URL.Hostname()), "api.anthropic.com") {
		path := strings.TrimSpace(req.URL.Path)
		switch {
		case req.Method == http.MethodPost && strings.HasPrefix(path, "/v1/messages/count_tokens"):
			return []string{
				"host",
				"connection",
				"accept",
				"x-stainless-retry-count",
				"x-stainless-lang",
				"x-stainless-package-version",
				"x-stainless-os",
				"x-stainless-arch",
				"x-stainless-runtime",
				"x-stainless-runtime-version",
				"anthropic-dangerous-direct-browser-access",
				"anthropic-version",
				"authorization",
				"x-app",
				"user-agent",
				"content-type",
				"anthropic-beta",
				"x-client-request-id",
				"accept-language",
				"sec-fetch-mode",
				"accept-encoding",
				"content-length",
			}
		case req.Method == http.MethodPost && strings.HasPrefix(path, "/v1/messages"):
			return []string{
				"host",
				"connection",
				"accept",
				"x-stainless-retry-count",
				"x-stainless-timeout",
				"x-stainless-lang",
				"x-stainless-package-version",
				"x-stainless-os",
				"x-stainless-arch",
				"x-stainless-runtime",
				"x-stainless-runtime-version",
				"anthropic-dangerous-direct-browser-access",
				"anthropic-version",
				"authorization",
				"x-app",
				"user-agent",
				"content-type",
				"anthropic-beta",
				"x-client-request-id",
				"accept-language",
				"sec-fetch-mode",
				"accept-encoding",
				"content-length",
			}
		case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/mcp_servers"):
			return []string{
				"accept",
				"content-type",
				"authorization",
				"anthropic-beta",
				"anthropic-version",
				"user-agent",
				"accept-encoding",
				"host",
				"connection",
			}
		case req.Method == http.MethodGet && strings.HasPrefix(path, "/api/oauth/usage"):
			return []string{
				"accept",
				"content-type",
				"user-agent",
				"authorization",
				"anthropic-beta",
				"accept-encoding",
				"host",
				"connection",
			}
		}
	}

	return []string{
		"host",
		"user-agent",
		"content-length",
		"accept",
		"accept-encoding",
		"accept-language",
		"anthropic-beta",
		"anthropic-dangerous-direct-browser-access",
		"anthropic-version",
		"authorization",
		"connection",
		"content-type",
		"sec-fetch-mode",
		"x-app",
		"x-client-request-id",
		"x-stainless-arch",
		"x-stainless-lang",
		"x-stainless-os",
		"x-stainless-package-version",
		"x-stainless-retry-count",
		"x-stainless-runtime",
		"x-stainless-runtime-version",
		"x-stainless-timeout",
		"x-stainless-helper-method",
	}
}
