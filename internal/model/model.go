package model

import (
	"sync"
	"time"
)

// Connection follows a Squid access.log record and appends passive fingerprint signals.
type Connection struct {
	ID                string    `json:"id"`
	Timestamp         time.Time `json:"timestamp"`
	ElapsedMS         int64     `json:"elapsed_ms"`
	ClientIP          string    `json:"client_ip"`
	Method            string    `json:"method"`
	URL               string    `json:"url"`
	Status            int       `json:"status"`
	Bytes             int64     `json:"bytes"`
	UserAgent         string    `json:"user_agent"`
	Protocol          string    `json:"protocol"`
	TLSVersion        string    `json:"tls_version"`
	ALPN              string    `json:"alpn"`
	ALPNOffered       string    `json:"alpn_offered,omitempty"`
	SNI               string    `json:"sni,omitempty"`
	ClientHelloSig    string    `json:"client_hello_signature,omitempty"`
	JA3               string    `json:"ja3"`
	JA4               string    `json:"ja4"`
	HTTP2Fingerprint  string    `json:"http2_fingerprint"`
	DetectedProfile   string    `json:"detected_profile"`
	UAConsistency     string    `json:"ua_consistency"`
	Risk              string    `json:"risk"`
	ReferenceProfile  string    `json:"reference_profile,omitempty"`
	ReferenceVersion  string    `json:"reference_version,omitempty"`
	ReferenceSource   string    `json:"reference_source,omitempty"`
	BaselineStatus    string    `json:"baseline_status,omitempty"`
	TLSClientHelloRaw string    `json:"tls_client_hello_raw,omitempty"`
}

type Store struct {
	mu    sync.RWMutex
	items []Connection
}

func NewStore(items []Connection) *Store { return &Store{items: items} }
func (s *Store) Add(c Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append([]Connection{c}, s.items...)
	if len(s.items) > 5000 {
		s.items = s.items[:5000]
	}
}
func (s *Store) List() []Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Connection(nil), s.items...)
}
