package fingerprint

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
)

func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func ProfileFromUA(ua string) string {
	lower := strings.ToLower(ua)
	switch {
	case strings.Contains(lower, "firefox"):
		return "Firefox-like"
	case strings.Contains(lower, "chrome"), strings.Contains(lower, "edg"):
		return "Chrome-like"
	case strings.Contains(lower, "safari"):
		return "Safari-like"
	case strings.Contains(lower, "curl"), strings.Contains(lower, "python"), strings.Contains(lower, "go-http-client"):
		return "Generic HTTP"
	default:
		return "Unknown"
	}
}
func TLSVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	default:
		return fmt.Sprintf("0x%x", v)
	}
}
func Stable(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, ",")))
	return hex.EncodeToString(h[:])[:32]
}
func Dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
