package demo

import (
	"httpfin/internal/fingerprint"
	"httpfin/internal/model"
	"time"
)

func Connections() []model.Connection {
	now := time.Now().UTC()
	return []model.Connection{
		{ID: "demo-1", Timestamp: now.Add(-20 * time.Second), ElapsedMS: 42, ClientIP: "185.220.101.42", Method: "GET", URL: "/api/health", Status: 200, Bytes: 1240, UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/122.0.0.0 Safari/537.36", Protocol: "HTTP/2.0", TLSVersion: "TLS 1.3", ALPN: "h2", JA3: fingerprint.Stable("chrome", "tls13"), JA4: "t13d1516h2_8daaf6152771_6c2c", HTTP2Fingerprint: "1:65536;2:0;3:1000;4:6291456", DetectedProfile: "Chrome-like", UAConsistency: "Matched", ObservationPoint: "demo"},
		{ID: "demo-2", Timestamp: now.Add(-2 * time.Minute), ElapsedMS: 87, ClientIP: "10.24.8.17", Method: "POST", URL: "/login", Status: 302, Bytes: 842, UserAgent: "Mozilla/5.0 Chrome/116.0.0.0", Protocol: "HTTP/2.0", TLSVersion: "TLS 1.3", ALPN: "h2", JA3: fingerprint.Stable("chrome116", "tls13"), JA4: "t13d1516h2_8daaf6152771_6c2c", HTTP2Fingerprint: "1:65536;2:0;3:1000;4:6291456", DetectedProfile: "Chrome-like", UAConsistency: "Matched", ObservationPoint: "demo"},
		{ID: "demo-3", Timestamp: now.Add(-4 * time.Minute), ElapsedMS: 121, ClientIP: "172.16.4.90", Method: "GET", URL: "/download/report.csv", Status: 200, Bytes: 91240, UserAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0", Protocol: "HTTP/2.0", TLSVersion: "TLS 1.3", ALPN: "h2", JA3: fingerprint.Stable("firefox121", "tls13"), JA4: "t13d1717h2_5b2c4d1e8b8e_7a19", HTTP2Fingerprint: "1:65536;2:0;4:131072", DetectedProfile: "Firefox-like", UAConsistency: "Matched", ObservationPoint: "demo"},
		{ID: "demo-4", Timestamp: now.Add(-7 * time.Minute), ElapsedMS: 311, ClientIP: "91.198.174.22", Method: "GET", URL: "/catalog?page=2", Status: 403, Bytes: 392, UserAgent: "python-requests/2.31.0", Protocol: "HTTP/1.1", TLSVersion: "TLS 1.3", ALPN: "http/1.1", JA3: fingerprint.Stable("python", "openssl"), JA4: "t13d191000_5b2c4d1e8b8e_0000", DetectedProfile: "Generic HTTP", UAConsistency: "Mismatch", ObservationPoint: "demo"},
	}
}
