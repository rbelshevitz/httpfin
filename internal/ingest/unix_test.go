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
	stop, err := StartUnixDatagram(path, store)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("sandbox disallows Unix datagram sockets")
		}
		t.Fatal(err)
	}
	defer stop()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o222 != 0o222 {
		t.Fatalf("unix datagram socket is not writable by nginx workers: mode %o", info.Mode().Perm())
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
	payload, _ := json.Marshal(nginxEvent{RequestID: "test-event", RemoteAddr: "127.0.0.1", Method: "GET", URI: "/health", Status: 200})
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
