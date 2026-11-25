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
	mu       sync.RWMutex
	items    []Connection
	next     int
	count    int
	capacity int
}

const defaultCapacity = 5000

func NewStore(items []Connection) *Store {
	s := &Store{capacity: defaultCapacity, items: make([]Connection, defaultCapacity)}
	for i := len(items) - 1; i >= 0; i-- {
		s.addLocked(items[i])
	}
	return s
}

func (s *Store) Add(c Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addLocked(c)
}

func (s *Store) addLocked(c Connection) {
	if s.capacity == 0 {
		s.capacity = defaultCapacity
		s.items = make([]Connection, s.capacity)
	}
	index := (s.next + s.count) % s.capacity
	if s.count == s.capacity {
		index = s.next
		s.next = (s.next + 1) % s.capacity
	} else {
		s.count++
	}
	s.items[index] = c
}
func (s *Store) List() []Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Connection, s.count)
	for i := 0; i < s.count; i++ {
		index := (s.next + s.count - 1 - i) % s.capacity
		result[i] = s.items[index]
	}
	return result
}
