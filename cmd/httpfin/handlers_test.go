package main

import (
	"encoding/json"
	"httpfin/internal/ingest"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIngestStatsHandler(t *testing.T) {
	counters := ingest.NewCounters()
	stop, healthy, err := ingest.StartUnixDatagramWithCounters("", nil, nil, counters)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	// Exercise the public JSON shape without depending on a socket in this
	// handler-level test.
	req := httptest.NewRequest(http.MethodGet, "/api/ingest", nil)
	res := httptest.NewRecorder()
	ingestStatsHandler(healthy, counters).ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	var got struct {
		Status  string                 `json:"status"`
		Healthy bool                   `json:"healthy"`
		Events  ingest.CounterSnapshot `json:"events"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "ok" || !got.Healthy || got.Events != (ingest.CounterSnapshot{}) {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestIngestStatsHandlerReturnsUnavailable(t *testing.T) {
	res := httptest.NewRecorder()
	ingestStatsHandler(func() bool { return false }, ingest.NewCounters()).ServeHTTP(
		res, httptest.NewRequest(http.MethodGet, "/api/ingest", nil),
	)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", res.Code)
	}
}
