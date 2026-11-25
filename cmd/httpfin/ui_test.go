package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebHandlerResetsIllustrativeFingerprintValues(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)
	webHandler().ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("got HTTP status %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"render(null);", "No observation selected", "Not observed", "HTTP/2 fingerprint"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("UI response does not contain %q", expected)
		}
	}
	for _, forbidden := range []string{"High confidence browser signature", "<b>94</b>", "TLS 1.3 <i class=\"check\">"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("UI response contains stale demo value %q", forbidden)
		}
	}
}
