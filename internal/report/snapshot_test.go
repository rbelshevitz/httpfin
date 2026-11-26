package report

import (
	"httpfin/internal/model"
	"testing"
	"time"
)

func TestBuildSnapshotSummarizesEvidenceAndFindings(t *testing.T) {
	rows := []model.Connection{
		{ID: "new", Timestamp: time.Unix(20, 0).UTC(), ClientIP: "203.0.113.8", UserAgent: "ExampleBrowser/1", Method: "GET", URL: "/api/items?token=secret", Status: 500, TLSVersion: "TLS 1.3", ALPN: "h2", BaselineStatus: "mismatch"},
		{ID: "old", Timestamp: time.Unix(10, 0).UTC(), ClientIP: "203.0.113.8", UserAgent: "ExampleBrowser/1", Method: "GET", URL: "/api/items?token=secret", Status: 200, TLSVersion: "TLS 1.2"},
	}
	s := BuildSnapshot(rows, time.Unix(30, 0))
	if s.Status != "Review" || s.Errors != 1 || s.Total != 2 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if s.Rate != .2 || s.Evidence.TLS != 2 || s.Evidence.ALPN != 1 {
		t.Fatalf("unexpected rate/evidence: %+v", s)
	}
	if len(s.Sources) != 1 || s.Sources[0].Count != 2 || len(s.Findings) != 1 {
		t.Fatalf("unexpected sources/findings: %+v", s)
	}
	if len(s.UserAgents) != 1 || s.UserAgents[0].UserAgent != "ExampleBrowser/1" || len(s.Destinations) != 1 || s.Destinations[0].Destination != "/api/items?token=%5BREDACTED%5D" {
		t.Fatalf("unexpected client/destination summaries: %+v %+v", s.UserAgents, s.Destinations)
	}
}

func TestBuildSnapshotEmptyState(t *testing.T) {
	s := BuildSnapshot(nil, time.Unix(30, 0))
	if s.Status != "Insufficient evidence" || s.Total != 0 || len(s.Findings) != 0 {
		t.Fatalf("unexpected empty snapshot: %+v", s)
	}
}

func TestBrowserFamilySeparatesPlatforms(t *testing.T) {
	for _, tc := range []struct{ ua, want string }{
		{"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/120 Mobile", "Android · Chrome"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1", "iOS · Safari"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120 Safari/537.36", "Chrome"},
	} {
		if got := browserFamily(tc.ua, ""); got != tc.want {
			t.Errorf("browserFamily(%q) = %q, want %q", tc.ua, got, tc.want)
		}
	}
}

func TestClientErrorsDoNotMarkServiceDegraded(t *testing.T) {
	s := BuildSnapshot([]model.Connection{{ID: "not-found", Timestamp: time.Unix(1, 0).UTC(), Status: 404}}, time.Unix(2, 0))
	if s.Status != "Normal" || s.Errors != 1 || s.ServerErrors != 0 {
		t.Fatalf("unexpected client-error state: %+v", s)
	}
}
