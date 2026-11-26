package report

import (
	"httpfin/internal/fingerprint"
	"httpfin/internal/model"
	"sort"
	"strings"
	"time"
)

type Snapshot struct {
	GeneratedAt  time.Time            `json:"generated_at"`
	WindowFrom   time.Time            `json:"window_from,omitempty"`
	WindowTo     time.Time            `json:"window_to,omitempty"`
	Total        int                  `json:"total"`
	Rate         float64              `json:"rate"`
	Status       string               `json:"status"`
	Statuses     []Count              `json:"statuses"`
	Errors       int                  `json:"errors"`
	ServerErrors int                  `json:"server_errors"`
	Sources      []SourceSummary      `json:"sources"`
	UserAgents   []UserAgentSummary   `json:"user_agents"`
	Destinations []DestinationSummary `json:"destinations"`
	Browsers     []BrowserSummary     `json:"browsers"`
	Routes       []RouteSummary       `json:"routes"`
	Findings     []Finding            `json:"findings"`
	Evidence     EvidenceSummary      `json:"evidence"`
	Observations []model.Connection   `json:"observations"`
	Recent       []model.Connection   `json:"recent"`
}

type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type SourceSummary struct {
	Source    string    `json:"source"`
	Count     int       `json:"count"`
	UserAgent string    `json:"user_agent,omitempty"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	Profile   string    `json:"profile,omitempty"`
	Evidence  string    `json:"evidence"`
	Baseline  string    `json:"baseline,omitempty"`
}

type UserAgentSummary struct {
	UserAgent   string    `json:"user_agent"`
	Profile     string    `json:"profile,omitempty"`
	Count       int       `json:"count"`
	Errors      int       `json:"errors"`
	SourceCount int       `json:"source_count"`
	LastSeen    time.Time `json:"last_seen"`
}

type DestinationSummary struct {
	Destination string    `json:"destination"`
	Count       int       `json:"count"`
	Errors      int       `json:"errors"`
	SourceCount int       `json:"source_count"`
	LastSeen    time.Time `json:"last_seen"`
}

type BrowserSummary struct {
	Family      string `json:"family"`
	Count       int    `json:"count"`
	SourceCount int    `json:"source_count"`
	Errors      int    `json:"errors"`
}

type RouteSummary struct {
	Source      string `json:"source"`
	UserAgent   string `json:"user_agent"`
	Profile     string `json:"profile,omitempty"`
	Destination string `json:"destination"`
	Count       int    `json:"count"`
	Errors      int    `json:"errors"`
}

type EvidenceSummary struct {
	TLS         int `json:"tls"`
	ALPN        int `json:"alpn"`
	ClientHello int `json:"client_hello"`
	JA3         int `json:"ja3"`
	JA4         int `json:"ja4"`
	HTTP2       int `json:"http2"`
}

type Finding struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Explanation string    `json:"explanation"`
	Count       int       `json:"count"`
	Source      string    `json:"source,omitempty"`
	FirstSeen   time.Time `json:"first_seen,omitempty"`
	LastSeen    time.Time `json:"last_seen,omitempty"`
	Action      string    `json:"action"`
}

func BuildSnapshot(rows []model.Connection, now time.Time) Snapshot {
	s := Snapshot{GeneratedAt: now.UTC(), Total: len(rows), Observations: rows}
	if len(rows) > 12 {
		s.Recent = rows[:12]
	} else {
		s.Recent = rows
	}
	if len(rows) == 0 {
		s.Status = "Insufficient evidence"
		return s
	}
	s.WindowTo = rows[0].Timestamp
	s.WindowFrom = rows[len(rows)-1].Timestamp
	if !s.WindowFrom.IsZero() && s.WindowTo.After(s.WindowFrom) {
		s.Rate = float64(len(rows)) / s.WindowTo.Sub(s.WindowFrom).Seconds()
	}
	status := map[string]int{}
	sources := map[string]*SourceSummary{}
	userAgents := map[string]*UserAgentSummary{}
	destinations := map[string]*DestinationSummary{}
	uaSources := map[string]map[string]struct{}{}
	destinationSources := map[string]map[string]struct{}{}
	browsers := map[string]*BrowserSummary{}
	browserSources := map[string]map[string]struct{}{}
	routes := map[string]*RouteSummary{}
	for _, c := range rows {
		class := statusClass(c.Status)
		status[class]++
		if c.Status >= 400 {
			s.Errors++
		}
		if c.Status >= 500 {
			s.ServerErrors++
		}
		if c.TLSVersion != "" {
			s.Evidence.TLS++
		}
		if c.ALPN != "" {
			s.Evidence.ALPN++
		}
		if c.ClientHelloSig != "" {
			s.Evidence.ClientHello++
		}
		if c.JA3 != "" {
			s.Evidence.JA3++
		}
		if c.JA4 != "" {
			s.Evidence.JA4++
		}
		if c.HTTP2Fingerprint != "" {
			s.Evidence.HTTP2++
		}
		key := fingerprint.Dash(c.ClientIP)
		if key == "unknown" {
			key = "unknown source"
		}
		entry := sources[key]
		if entry == nil {
			entry = &SourceSummary{Source: key, FirstSeen: c.Timestamp, LastSeen: c.Timestamp}
			sources[key] = entry
		}
		entry.Count++
		if c.Timestamp.Before(entry.FirstSeen) {
			entry.FirstSeen = c.Timestamp
		}
		if c.Timestamp.After(entry.LastSeen) {
			entry.LastSeen = c.Timestamp
		}
		if entry.Profile == "" {
			entry.Profile = c.DetectedProfile
		}
		if entry.UserAgent == "" {
			entry.UserAgent = c.UserAgent
		}
		if entry.Baseline == "" {
			entry.Baseline = c.BaselineStatus
		}
		entry.Evidence = evidenceState(c)
		ua := c.UserAgent
		if ua == "" {
			ua = "Unknown"
		}
		uaEntry := userAgents[ua]
		if uaEntry == nil {
			uaEntry = &UserAgentSummary{UserAgent: ua, Profile: c.DetectedProfile, LastSeen: c.Timestamp}
			userAgents[ua] = uaEntry
			uaSources[ua] = map[string]struct{}{}
		}
		uaEntry.Count++
		if c.Status >= 400 {
			uaEntry.Errors++
		}
		if c.Timestamp.After(uaEntry.LastSeen) {
			uaEntry.LastSeen = c.Timestamp
		}
		uaSources[ua][key] = struct{}{}
		destination := redactURL(c.URL)
		destinationEntry := destinations[destination]
		if destinationEntry == nil {
			destinationEntry = &DestinationSummary{Destination: destination, LastSeen: c.Timestamp}
			destinations[destination] = destinationEntry
			destinationSources[destination] = map[string]struct{}{}
		}
		destinationEntry.Count++
		if c.Status >= 400 {
			destinationEntry.Errors++
		}
		if c.Timestamp.After(destinationEntry.LastSeen) {
			destinationEntry.LastSeen = c.Timestamp
		}
		destinationSources[destination][key] = struct{}{}
		browser := browserFamily(c.UserAgent, c.DetectedProfile)
		browserEntry := browsers[browser]
		if browserEntry == nil {
			browserEntry = &BrowserSummary{Family: browser}
			browsers[browser] = browserEntry
			browserSources[browser] = map[string]struct{}{}
		}
		browserEntry.Count++
		if c.Status >= 400 {
			browserEntry.Errors++
		}
		browserSources[browser][key] = struct{}{}
		routeKey := key + "\x00" + ua + "\x00" + destination
		route := routes[routeKey]
		if route == nil {
			route = &RouteSummary{Source: key, UserAgent: ua, Profile: c.DetectedProfile, Destination: destination}
			routes[routeKey] = route
		}
		route.Count++
		if c.Status >= 400 {
			route.Errors++
		}
	}
	for k, v := range status {
		s.Statuses = append(s.Statuses, Count{k, v})
	}
	sort.Slice(s.Statuses, func(i, j int) bool { return s.Statuses[i].Name < s.Statuses[j].Name })
	for _, v := range sources {
		s.Sources = append(s.Sources, *v)
	}
	sort.Slice(s.Sources, func(i, j int) bool { return s.Sources[i].Count > s.Sources[j].Count })
	for key, entry := range userAgents {
		entry.SourceCount = len(uaSources[key])
		s.UserAgents = append(s.UserAgents, *entry)
	}
	for key, entry := range destinations {
		entry.SourceCount = len(destinationSources[key])
		s.Destinations = append(s.Destinations, *entry)
	}
	sort.Slice(s.UserAgents, func(i, j int) bool { return s.UserAgents[i].Count > s.UserAgents[j].Count })
	sort.Slice(s.Destinations, func(i, j int) bool { return s.Destinations[i].Count > s.Destinations[j].Count })
	for key, entry := range browsers {
		entry.SourceCount = len(browserSources[key])
		s.Browsers = append(s.Browsers, *entry)
	}
	sort.Slice(s.Browsers, func(i, j int) bool { return s.Browsers[i].Count > s.Browsers[j].Count })
	for _, route := range routes {
		s.Routes = append(s.Routes, *route)
	}
	sort.Slice(s.Routes, func(i, j int) bool { return s.Routes[i].Count > s.Routes[j].Count })
	if len(s.Routes) > 12 {
		s.Routes = s.Routes[:12]
	}
	s.Findings = findings(rows)
	if len(s.Findings) > 0 {
		s.Status = "Review"
	} else if s.ServerErrors > 0 {
		s.Status = "Degraded"
	} else {
		s.Status = "Normal"
	}
	return s
}

func browserFamily(userAgent, profile string) string {
	value := strings.ToLower(userAgent + " " + profile)
	platform := ""
	if strings.Contains(value, "android") {
		platform = "Android · "
	} else if strings.Contains(value, "iphone") || strings.Contains(value, "ipad") || strings.Contains(value, "ipod") || strings.Contains(value, "ios") {
		platform = "iOS · "
	}
	switch {
	case strings.Contains(value, "edg") || strings.Contains(value, "edge"):
		return platform + "Edge"
	case strings.Contains(value, "opr") || strings.Contains(value, "opera"):
		return platform + "Opera"
	case strings.Contains(value, "firefox"):
		return platform + "Firefox"
	case strings.Contains(value, "chrome") || strings.Contains(value, "chromium"):
		return platform + "Chrome"
	case strings.Contains(value, "safari") && !strings.Contains(value, "chrome"):
		return platform + "Safari"
	case strings.Contains(value, "curl"):
		return platform + "curl"
	case strings.Contains(value, "wget"):
		return platform + "Wget"
	case strings.Contains(value, "postman"):
		return platform + "Postman"
	default:
		if platform != "" {
			return strings.TrimSpace(strings.TrimSuffix(platform, "· "))
		}
		return "Other"
	}
}

func findings(rows []model.Connection) []Finding {
	var out []Finding
	for _, c := range rows {
		if c.BaselineStatus == "mismatch" {
			out = append(out, Finding{ID: "baseline-" + c.ID, Kind: "baseline_mismatch", Severity: "high", Title: "Baseline mismatch", Explanation: "Observed signature differs from the selected baseline.", Count: 1, Source: fingerprint.Dash(c.ClientIP), FirstSeen: c.Timestamp, LastSeen: c.Timestamp, Action: "Compare the request with the expected client and review the observation point."})
		}
	}
	return out
}

func statusClass(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return "other"
	}
}
