package ingest

import (
	"encoding/json"
	"fmt"
	"httpfin/internal/model"
	"log"
	"net"
	"os"
	"os/user"
	"strconv"
	"sync/atomic"
	"time"
)

const (
	maxRequestID = 256
	maxField     = 16 * 1024
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
func StartUnixDatagram(path string, store *model.Store) (func(), func() bool, error) {
	if path == "" {
		return func() {}, func() bool { return true }, nil
	}
	_ = os.Remove(path)
	addr, err := net.ResolveUnixAddr("unixgram", path)
	if err != nil {
		return nil, nil, err
	}
	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		return nil, nil, err
	}
	// The socket is intentionally group-writable, not world-writable. Set
	// httpfin_socket_group to the Nginx worker group when the processes do not
	// share the same primary group.
	if err := os.Chmod(path, 0o660); err != nil {
		_ = conn.Close()
		_ = os.Remove(path)
		return nil, nil, err
	}
	if group := os.Getenv("httpfin_socket_group"); group != "" {
		gid, err := lookupGroupID(group)
		if err != nil {
			_ = conn.Close()
			_ = os.Remove(path)
			return nil, nil, err
		}
		if err := os.Chown(path, -1, gid); err != nil {
			_ = conn.Close()
			_ = os.Remove(path)
			return nil, nil, err
		}
	}
	stop := make(chan struct{})
	var healthy atomic.Bool
	healthy.Store(true)
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
				healthy.Store(false)
				log.Printf("httpfin ingest stopped: %v", readErr)
				return
			}
			var wire nginxEvent
			if err := json.Unmarshal(buf[:n], &wire); err != nil {
				continue
			}
			if !validEvent(wire) {
				continue
			}
			event := model.Connection{ID: wire.RequestID, Timestamp: time.Now().UTC(), ClientIP: wire.RemoteAddr, Method: wire.Method, URL: wire.URI, Status: wire.Status, UserAgent: wire.UserAgent, TLSVersion: wire.TLSVersion, SNI: wire.SNI, ALPN: wire.ALPN, DetectedProfile: "Unknown", UAConsistency: "Unknown", Risk: "Info", Protocol: "HTTP"}
			if event.ID == "" {
				event.ID = fmt.Sprintf("nginx-%d", n)
			}
			store.Add(event)
		}
	}()
	return func() { close(stop); _ = conn.Close(); _ = os.Remove(path) }, healthy.Load, nil
}

func lookupGroupID(group string) (int, error) {
	if gid, err := strconv.Atoi(group); err == nil && gid >= 0 {
		return gid, nil
	}
	entry, err := user.LookupGroup(group)
	if err != nil {
		return 0, fmt.Errorf("lookup socket group %q: %w", group, err)
	}
	gid, err := strconv.Atoi(entry.Gid)
	if err != nil {
		return 0, fmt.Errorf("invalid gid for socket group %q: %w", group, err)
	}
	return gid, nil
}

func validEvent(w nginxEvent) bool {
	return w.Version == 1 && len(w.RequestID) <= maxRequestID &&
		len(w.Method) <= maxField && len(w.URI) <= maxField &&
		len(w.Host) <= maxField && len(w.UserAgent) <= maxField &&
		len(w.RemoteAddr) <= maxField && len(w.TLSVersion) <= maxField &&
		len(w.SNI) <= maxField && len(w.ALPN) <= maxField
}

// deadline is kept short so shutdown is responsive without introducing a
// separate wakeup socket.
func deadline() time.Time { return time.Now().Add(500 * time.Millisecond) }
