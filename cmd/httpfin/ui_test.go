package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebHandlerRendersInvestigationDashboard(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)
	webHandler().ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("got HTTP status %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"httpfin — observations", "No observations yet", "api/snapshot", "inspect"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("UI response does not contain %q: %s", expected, body)
		}
	}
	for _, forbidden := range []string{"High confidence browser signature", "<b>94</b>", "Chrome-like"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("UI response contains stale demo value %q", forbidden)
		}
	}
}
