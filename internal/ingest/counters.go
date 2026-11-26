package ingest

import "sync/atomic"

// Counters describes the lifecycle of events received from the Nginx module.
// The counters cover the Unix-datagram ingest path only; direct Go HTTP/TLS
// observations are already stored without passing through this pipeline.
type Counters struct {
	received atomic.Uint64
	decoded  atomic.Uint64
	stored   atomic.Uint64
	dropped  atomic.Uint64
}

type CounterSnapshot struct {
	Received uint64 `json:"received"`
	Decoded  uint64 `json:"decoded"`
	Stored   uint64 `json:"stored"`
	Dropped  uint64 `json:"dropped"`
}

func NewCounters() *Counters { return &Counters{} }

func (c *Counters) Snapshot() CounterSnapshot {
	if c == nil {
		return CounterSnapshot{}
	}
	return CounterSnapshot{
		Received: c.received.Load(),
		Decoded:  c.decoded.Load(),
		Stored:   c.stored.Load(),
		Dropped:  c.dropped.Load(),
	}
}
