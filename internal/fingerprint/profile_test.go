package fingerprint

import "testing"

func TestProfileFromUADistinguishesCommonBrowsers(t *testing.T) {
	tests := []struct {
		name, ua, want string
	}{
		{"chrome", "Mozilla/5.0 Chrome/122.0.0.0 Safari/537.36", "Chrome-like"},
		{"edge", "Mozilla/5.0 Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0", "Edge-like"},
		{"edge-android", "Mozilla/5.0 Android 14 Chrome/122.0.0.0 Mobile Safari/537.36 EdgA/122.0.0.0", "Edge Android-like"},
		{"edge-ios", "Mozilla/5.0 CriOS/122.0.0.0 EdgiOS/122.0.0.0 Mobile", "Edge iOS-like"},
		{"firefox", "Mozilla/5.0 Firefox/123.0", "Firefox-like"},
		{"firefox-ios", "Mozilla/5.0 FxiOS/123.0 Mobile", "Firefox iOS-like"},
		{"opera", "Mozilla/5.0 Chrome/122.0.0.0 OPR/108.0.0.0", "Opera-like"},
		{"opera-mini", "Opera Mini/36.2.2254/191.255 Android", "Opera Mobile-like"},
		{"chrome-ios", "Mozilla/5.0 CriOS/122.0.0.0 Mobile", "Chrome iOS-like"},
		{"chrome-android", "Mozilla/5.0 Android 14 Chrome/122.0.0.0 Mobile Safari/537.36", "Chrome Android-like"},
		{"safari", "Mozilla/5.0 Version/17.0 Mobile/15E148 Safari/604.1", "Safari-like"},
		{"samsung", "Mozilla/5.0 Android 14 Chrome/122.0.0.0 Mobile Safari/537.36 SamsungBrowser/26.0", "Samsung Internet-like"},
		{"webview", "Mozilla/5.0 (Linux; Android 14; Pixel) AppleWebKit/537.36 Version/4.0 Chrome/122.0.0.0 Mobile Safari/537.36; wv", "Android WebView-like"},
		{"yandex", "Mozilla/5.0 Android Chrome/122.0.0.0 YaBrowser/24.1 Mobile Safari/537.36", "Yandex Browser-like"},
		{"uc", "Mozilla/5.0 Android UCBrowser/15.0 Mobile Safari/537.36", "UC Browser-like"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProfileFromUA(tt.ua); got != tt.want {
				t.Fatalf("ProfileFromUA() = %q, want %q", got, tt.want)
			}
		})
	}
}
