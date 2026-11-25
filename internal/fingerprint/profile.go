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
	case strings.Contains(lower, "; wv)") || strings.Contains(lower, "; wv;") || strings.HasSuffix(lower, "; wv"):
		return "Android WebView-like"
	case strings.Contains(lower, "samsungbrowser"):
		return "Samsung Internet-like"
	case strings.Contains(lower, "yabrowser"):
		return "Yandex Browser-like"
	case strings.Contains(lower, "ucbrowser") || strings.Contains(lower, "ucweb"):
		return "UC Browser-like"
	case strings.Contains(lower, "huaweibrowser"):
		return "Huawei Browser-like"
	case strings.Contains(lower, "miuibrowser"):
		return "Mi Browser-like"
	case strings.Contains(lower, "duckduckgo"):
		return "DuckDuckGo-like"
	case strings.Contains(lower, "fban") || strings.Contains(lower, "fbav") || strings.Contains(lower, "instagram") || strings.Contains(lower, "line/"):
		return "In-App WebView-like"
	case strings.Contains(lower, "edgios"):
		return "Edge iOS-like"
	case strings.Contains(lower, "edga/"):
		return "Edge Android-like"
	case strings.Contains(lower, "edg/"):
		return "Edge-like"
	case strings.Contains(lower, "opera mini") || strings.Contains(lower, "opera mobi"):
		return "Opera Mobile-like"
	case strings.Contains(lower, "opr/") || strings.Contains(lower, "opera"):
		return "Opera-like"
	case strings.Contains(lower, "fxios"):
		return "Firefox iOS-like"
	case strings.Contains(lower, "crios"):
		return "Chrome iOS-like"
	case strings.Contains(lower, "firefox"):
		return "Firefox-like"
	case strings.Contains(lower, "chrome"):
		if strings.Contains(lower, "android") {
			return "Chrome Android-like"
		}
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
