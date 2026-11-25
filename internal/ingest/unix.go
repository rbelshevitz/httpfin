package ingest

import (
	"encoding/json"
	"fmt"
	"httpfin/internal/model"
	"net"
	"os"
	"time"
)

type nginxEvent struct {
	Version    int    `json:"version"`
	RequestID  string `json:"request_id"`
	Method     string `json:"method"`
	URI        string `json:"uri"`
	Host       string `json:"host"`
	UserAgent  string `json:"user_agent"`
	RemoteAddr string `json:"remote_addr"`
	Status     int    `json:"status"`
	TLSVersion string `json:"tls_version"`
	SNI        string `json:"sni"`
	ALPN       string `json:"alpn"`
}

// StartUnixDatagram consumes compact JSON events from an Nginx module.
// Datagram writes are intentionally one-way: an unavailable collector must
// not stall an Nginx request worker.
func StartUnixDatagram(path string, store *model.Store) (func(), error) {
	if path == "" {
		return func() {}, nil
	}
	_ = os.Remove(path)
	addr, err := net.ResolveUnixAddr("unixgram", path)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		return nil, err
	}
	// Nginx workers normally run as an unprivileged user (for example,
	// "nobody" in the Alpine image).  Let them write telemetry datagrams
	// while keeping the socket local to the container/host filesystem.
	if err := os.Chmod(path, 0o666); err != nil {
		_ = conn.Close()
		_ = os.Remove(path)
		return nil, err
	}
	stop := make(chan struct{})
	go func() {
		buf := make([]byte, 64*1024)
		for {
			_ = conn.SetReadDeadline(deadline())
			n, _, readErr := conn.ReadFromUnix(buf)
			select {
			case <-stop:
				return
			default:
			}
			if readErr != nil {
				if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}
			var wire nginxEvent
			if err := json.Unmarshal(buf[:n], &wire); err != nil {
				continue
			}
			event := model.Connection{ID: wire.RequestID, Timestamp: time.Now().UTC(), ClientIP: wire.RemoteAddr, Method: wire.Method, URL: wire.URI, Status: wire.Status, UserAgent: wire.UserAgent, TLSVersion: wire.TLSVersion, SNI: wire.SNI, ALPN: wire.ALPN, DetectedProfile: "Unknown", UAConsistency: "Unknown", Risk: "Info", Protocol: "HTTP"}
			if event.ID == "" {
				event.ID = fmt.Sprintf("nginx-%d", n)
			}
			store.Add(event)
		}
	}()
	return func() { close(stop); _ = conn.Close(); _ = os.Remove(path) }, nil
}

// deadline is kept short so shutdown is responsive without introducing a
// separate wakeup socket.
func deadline() time.Time { return time.Now().Add(500 * time.Millisecond) }
