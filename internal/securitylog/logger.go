package securitylog

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	maxIP       = 64
	maxEndpoint = 256
)

// Logger writes a deliberately small, line-oriented stream for Fail2ban or a
// similar host-side consumer. An empty path disables the stream.
type Logger struct {
	mu sync.Mutex
	f  *os.File
}

func New(path string) (*Logger, error) {
	if path == "" {
		return &Logger{}, nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, err
	}
	return &Logger{f: f}, nil
}

func (l *Logger) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}

// RecordRequest emits only statuses that commonly represent an actionable
// security failure. It intentionally ignores 4xx values that are too noisy for
// a default ban signal.
func (l *Logger) RecordRequest(remoteAddr, requestURI string, status int) error {
	reason, event, ok := eventForStatus(status)
	if !ok || l == nil || l.f == nil {
		return nil
	}
	ip := normalizeIP(remoteAddr)
	endpoint := normalizeEndpoint(requestURI)
	line := fmt.Sprintf("httpfin-security ts=%s src=%s event=%s endpoint=%s status=%d reason=%s\n",
		time.Now().UTC().Format(time.RFC3339Nano), ip, event, endpoint, status, reason)
	return l.writeLine(line)
}

// RecordBotDecision emits a policy result. Fail2ban filters should match only
// decision=deny; observe and review are evidence for operators, not bans.
func (l *Logger) RecordBotDecision(remoteAddr, requestURI, action, reason string, confidence float64) error {
	if l == nil || l.f == nil || action == "" || reason == "" {
		return nil
	}
	if action != "observe" && action != "review" && action != "challenge" && action != "deny" {
		return nil
	}
	ip := normalizeIP(remoteAddr)
	endpoint := normalizeEndpoint(requestURI)
	reason = normalizeToken(reason, "unknown")
	line := fmt.Sprintf("httpfin-security ts=%s src=%s event=bot_suspected endpoint=%s decision=%s confidence=%.2f reason=%s\n",
		time.Now().UTC().Format(time.RFC3339Nano), ip, endpoint, action, confidence, reason)
	return l.writeLine(line)
}

func (l *Logger) writeLine(line string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, err := l.f.WriteString(line)
	return err
}

func normalizeToken(value, fallback string) string {
	if value == "" {
		return fallback
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return truncate(b.String(), 64)
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func eventForStatus(status int) (reason, event string, ok bool) {
	switch status {
	case http.StatusUnauthorized:
		return "invalid_credentials", "auth_failure", true
	case http.StatusForbidden:
		return "access_denied", "access_denied", true
	case http.StatusTooManyRequests:
		return "rate_limit", "rate_limit", true
	default:
		return "", "", false
	}
}

func normalizeIP(value string) string {
	host := value
	if parsed, _, err := net.SplitHostPort(value); err == nil {
		host = parsed
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return "-"
}

func normalizeEndpoint(value string) string {
	if index := strings.IndexByte(value, '?'); index >= 0 {
		value = value[:index]
	}
	if value == "" {
		value = "/"
	}
	if len(value) > maxEndpoint {
		value = value[:maxEndpoint]
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if r <= ' ' || r == '=' || r == '\\' {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
