package report

import (
	"httpfin/internal/model"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWriteExplainsClientsBotSignalsAndFingerprintChanges(t *testing.T) {
	rows := []model.Connection{
		{ID: "new", Timestamp: time.Unix(2, 0).UTC(), ClientIP: "203.0.113.8", Method: "GET", URL: "/v1/data?token=secret&client_secret=also-secret&view=full", Status: 200, Protocol: "HTTP/2", TLSVersion: "TLS 1.3", ALPN: "h2", ClientHelloSig: "hello-new", JA3: "ja3-new", HTTP2Fingerprint: "h2-new", DetectedProfile: "Chrome-like", UserAgent: "Chrome", Risk: "Review", UAConsistency: "Mismatch", BaselineStatus: "mismatch", ReferenceProfile: "Chrome"},
		{ID: "old", Timestamp: time.Unix(1, 0).UTC(), ClientIP: "203.0.113.8", Method: "GET", URL: "/v1/data", Status: 200, Protocol: "HTTP/2", TLSVersion: "TLS 1.2", ALPN: "h2", ClientHelloSig: "hello-old", JA3: "ja3-old", HTTP2Fingerprint: "h2-old", DetectedProfile: "Chrome-like"},
		{ID: "other", Timestamp: time.Unix(1, 0).UTC(), ClientIP: "198.51.100.5", Method: "GET", URL: "/health", Status: 204, DetectedProfile: "Unknown"},
	}
	recorder := httptest.NewRecorder()
	Write(recorder, rows)
	body := recorder.Body.String()
	for _, expected := range []string{
		"CLIENTS AND BOT SIGNALS",
		"203.0.113.8",
		"suspected",
		"not_suspected",
		"fingerprint_change: tls:TLS 1.2->TLS 1.3",
		"ja3:ja3-old->ja3-new",
		"%5BREDACTED%5D",
		"insufficient independent signals",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("report does not contain %q:\n%s", expected, body)
		}
	}
	if strings.Contains(body, "token=secret") {
		t.Fatal("report leaked a sensitive query value")
	}
	if strings.Contains(body, "client_secret=also-secret") {
		t.Fatal("report leaked a client secret query value")
	}
}

func TestFingerprintChangesWithoutPreviousObservation(t *testing.T) {
	got := fingerprintChanges(model.Connection{TLSVersion: "TLS 1.3"}, nil)
	if got != "no_previous_observation" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteCreatesUsefulEmptyReport(t *testing.T) {
	recorder := httptest.NewRecorder()
	Write(recorder, nil)
	body := recorder.Body.String()
	if recorder.Code != 200 {
		t.Fatalf("got HTTP status %d", recorder.Code)
	}
	for _, expected := range []string{"Observations: 0", "CLIENTS AND BOT SIGNALS", "No observations yet."} {
		if !strings.Contains(body, expected) {
			t.Fatalf("empty report does not contain %q:\n%s", expected, body)
		}
	}
	if got := recorder.Header().Get("Content-Disposition"); got != `attachment; filename="httpfin-report.txt"` {
		t.Fatalf("unexpected content disposition: %q", got)
	}
}

func TestWriteBoundsLongDiagnosticFields(t *testing.T) {
	recorder := httptest.NewRecorder()
	Write(recorder, []model.Connection{{
		ID: "long", ClientIP: "203.0.113.9", Method: "GET", URL: "/",
		Status: 200, ClientHelloSig: strings.Repeat("x", 10000),
		JA3: strings.Repeat("y", 10000), HTTP2Fingerprint: strings.Repeat("z", 10000),
	}})
	body := recorder.Body.String()
	if strings.Contains(body, strings.Repeat("x", 1000)) || strings.Contains(body, strings.Repeat("y", 1000)) || strings.Contains(body, strings.Repeat("z", 1000)) {
		t.Fatal("report emitted an unbounded diagnostic field")
	}
	if !strings.Contains(body, "client_hello=") || !strings.Contains(body, "ja3=") || !strings.Contains(body, "http2=") {
		t.Fatal("report omitted bounded diagnostic fields")
	}
}

func BenchmarkWriteReport5000(b *testing.B) {
	rows := make([]model.Connection, 5000)
	for i := range rows {
		rows[i] = model.Connection{
			ID:               "request",
			Timestamp:        time.Unix(int64(i), 0).UTC(),
			ClientIP:         "203.0.113.10",
			Method:           "GET",
			URL:              "/v1/health?view=summary",
			Status:           200,
			Protocol:         "HTTP/2",
			ObservationPoint: "nginx-http-module",
			TLSVersion:       "TLS 1.3",
			ALPN:             "h2",
			DetectedProfile:  "Chrome Android-like",
			UAConsistency:    "Unknown",
			Risk:             "Info",
			HTTP2Fingerprint: "settings",
			ClientHelloSig:   "hello",
		}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Write(httptest.NewRecorder(), rows)
	}
}
