package securitylog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordRequestWritesSafeEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "security.log")
	logger, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.RecordRequest("203.0.113.8:443", "/v1/login?token=secret", 401); err != nil {
		t.Fatal(err)
	}
	if err := logger.RecordRequest("203.0.113.8:443", "/health", 200); err != nil {
		t.Fatal(err)
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	if !strings.Contains(line, "src=203.0.113.8 event=auth_failure endpoint=/v1/login status=401 reason=invalid_credentials") {
		t.Fatalf("unexpected event: %q", line)
	}
	if strings.Contains(line, "token=secret") || strings.Count(line, "httpfin-security") != 1 {
		t.Fatalf("security log leaked data or wrote noise: %q", line)
	}
}

func TestNormalizeIPRejectsUntrustedText(t *testing.T) {
	if got := normalizeIP("198.51.100.4:1234"); got != "198.51.100.4" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeIP("attacker\n198.51.100.4"); got != "-" {
		t.Fatalf("got %q", got)
	}
}

func TestRecordBotDecisionUsesSafeFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "security.log")
	logger, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.RecordBotDecision("203.0.113.8:443", "/v1/data?token=secret", "observe", "ua_tls_baseline_mismatch", 0.95); err != nil {
		t.Fatal(err)
	}
	_ = logger.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	if !strings.Contains(line, "event=bot_suspected") || !strings.Contains(line, "decision=observe") {
		t.Fatalf("unexpected bot event: %q", line)
	}
	if strings.Contains(line, "token=secret") {
		t.Fatalf("query string leaked: %q", line)
	}
}
