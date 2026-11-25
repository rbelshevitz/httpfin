package ingest

import (
	"encoding/json"
	"httpfin/internal/model"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartUnixDatagram(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.sock")
	store := model.NewStore(nil)
	stop, healthy, err := StartUnixDatagram(path, store)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("sandbox disallows Unix datagram sockets")
		}
		t.Fatal(err)
	}
	defer stop()
	if !healthy() {
		t.Fatal("ingest should be healthy after start")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o660 {
		t.Fatalf("unix datagram socket has unexpected permissions: mode %o", info.Mode().Perm())
	}
	addr, err := net.ResolveUnixAddr("unixgram", path)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUnix("unixgram", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	payload, _ := json.Marshal(nginxEvent{Version: 1, RequestID: "test-event", RemoteAddr: "127.0.0.1", Method: "GET", URI: "/health", Status: 200})
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := store.List(); len(got) == 1 && got[0].ID == "test-event" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("event was not ingested: %#v", store.List())
}

func TestStartUnixDatagramWithHandler(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.sock")
	store := model.NewStore(nil)
	seen := make(chan model.Connection, 1)
	stop, _, err := StartUnixDatagramWithHandler(path, store, func(event model.Connection) {
		seen <- event
	})
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
	payload, _ := json.Marshal(nginxEvent{Version: 1, RequestID: "pipeline-event", RemoteAddr: "203.0.113.10", Method: "GET", URI: "/api/items", Status: 200, UserAgent: "Mozilla/5.0"})
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-seen:
		if event.ID != "pipeline-event" || event.ClientIP != "203.0.113.10" {
			t.Fatalf("unexpected handler event: %#v", event)
		}
		if got := store.List(); len(got) != 0 {
			t.Fatalf("handler mode must leave storage to the pipeline, got %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not receive event")
	}
}

func FuzzNginxEventDecoder(f *testing.F) {
	f.Add([]byte(`{"version":1,"request_id":"seed","method":"GET","uri":"/health","status":200}`))
	f.Add([]byte(`{`))
	f.Add([]byte{})
	f.Add([]byte(`{"version":2,"request_id":"wrong-version"}`))

	f.Fuzz(func(t *testing.T, payload []byte) {
		// Datagram payloads are bounded by the module, and the decoder must keep
		// the same bound even when called directly by a test or future adapter.
		if len(payload) > 64*1024 {
			payload = payload[:64*1024]
		}
		var event nginxEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return
		}
		_ = validEvent(event)
	})
}

func TestValidEventRejectsOversizedFields(t *testing.T) {
	event := nginxEvent{Version: 1, RequestID: "ok", Method: "GET", URI: "/"}
	if !validEvent(event) {
		t.Fatal("baseline event should be valid")
	}
	event.UserAgent = strings.Repeat("x", maxField+1)
	if validEvent(event) {
		t.Fatal("oversized user-agent should be rejected")
	}
}

func TestStartUnixDatagramSkipsInvalidAndContinues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.sock")
	store := model.NewStore(nil)
	stop, healthy, err := StartUnixDatagram(path, store)
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

	invalid := [][]byte{
		[]byte("{"),
		[]byte(`{"version":2,"request_id":"bad","method":"GET","uri":"/"}`),
		[]byte(`{"version":1,"request_id":"bad","method":"GET","uri":"` + strings.Repeat("x", maxField+1) + `"}`),
	}
	for _, payload := range invalid {
		if _, err := conn.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	valid, _ := json.Marshal(nginxEvent{Version: 1, RequestID: "after-invalid", Method: "GET", URI: "/ok"})
	if _, err := conn.Write(valid); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := store.List(); len(got) == 1 && got[0].ID == "after-invalid" {
			if !healthy() {
				t.Fatal("ingest became unhealthy after invalid datagrams")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("invalid datagrams were not ignored or valid event was not ingested: %#v", store.List())
}

func TestStartUnixDatagramCanRestartOnSamePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.sock")
	firstStore := model.NewStore(nil)
	stopFirst, _, err := StartUnixDatagram(path, firstStore)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("sandbox disallows Unix datagram sockets")
		}
		t.Fatal(err)
	}
	stopFirst()

	secondStore := model.NewStore(nil)
	stopSecond, healthy, err := StartUnixDatagram(path, secondStore)
	if err != nil {
		t.Fatal("collector did not restart on the same socket path:", err)
	}
	defer stopSecond()
	if !healthy() {
		t.Fatal("restarted collector should be healthy")
	}

	addr, err := net.ResolveUnixAddr("unixgram", path)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUnix("unixgram", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	payload, _ := json.Marshal(nginxEvent{Version: 1, RequestID: "after-restart", Method: "GET", URI: "/ready"})
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := secondStore.List(); len(got) == 1 && got[0].ID == "after-restart" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("restarted collector did not ingest event: %#v", secondStore.List())
}
