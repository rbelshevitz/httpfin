package tests

import (
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"httpfin/internal/ingest"
	"httpfin/internal/model"
)

// TestNginxUnixDatagramFormat exercises the wire format documented by the
// native module without requiring NGINX to be compiled. Keep this payload
// byte-for-byte compatible with ngx_http_httpfin_emit.
func TestNginxUnixDatagramFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.sock")
	store := model.NewStore(nil)
	stop, _, err := ingest.StartUnixDatagram(path, store)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("sandbox disallows Unix datagram sockets")
		}
		t.Fatal(err)
	}
	defer stop()

	addr, err := net.ResolveUnixAddr("unixgram", path)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUnix("unixgram", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	payload := []byte(`{"version":1,"request_id":"worker-17","method":"GET","uri":"/api/items?q=a%20b","host":"inspector.example.test","user_agent":"curl/8.10.1","remote_addr":"192.0.2.10","status":204,"tls_version":"TLS 1.3","sni":"inspector.example.test","alpn":"h2"}` + "\n")
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}

	var got model.Connection
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		items := store.List()
		if len(items) == 1 {
			got = items[0]
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got.ID == "" {
		t.Fatalf("event was not ingested: %#v", store.List())
	}
	if got.ID != "worker-17" || got.Method != "GET" || got.URL != "/api/items?q=a%20b" ||
		got.ClientIP != "192.0.2.10" || got.Status != 204 || got.UserAgent != "curl/8.10.1" ||
		got.TLSVersion != "TLS 1.3" || got.SNI != "inspector.example.test" || got.ALPN != "h2" {
		t.Fatalf("decoded event does not match NGINX format: %+v", got)
	}
}

func TestNginxUnixDatagramFormatIgnoresMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.sock")
	store := model.NewStore(nil)
	stop, _, err := ingest.StartUnixDatagram(path, store)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("sandbox disallows Unix datagram sockets")
		}
		t.Fatal(err)
	}
	defer stop()

	addr, err := net.ResolveUnixAddr("unixgram", path)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUnix("unixgram", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("not-json\n")); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)
	if got := store.List(); len(got) != 0 {
		t.Fatalf("malformed datagram was accepted: %#v", got)
	}
}

func TestNginxUnixDatagramFormatPayloadIsValidJSON(t *testing.T) {
	var wire map[string]any
	payload := []byte(`{"version":1,"request_id":"worker-17","method":"GET","uri":"/healthz","host":"example.test","user_agent":"curl/8","remote_addr":"127.0.0.1","status":200}`)
	if err := json.Unmarshal(payload, &wire); err != nil {
		t.Fatalf("fixture is not valid JSON: %v", err)
	}
	if wire["version"] != float64(1) || wire["request_id"] != "worker-17" {
		t.Fatalf("unexpected format fields: %#v", wire)
	}
}
