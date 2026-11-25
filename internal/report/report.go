package report

import (
	"fmt"
	"httpfin/internal/fingerprint"
	"httpfin/internal/model"
	"httpfin/internal/policy"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Write creates a human-readable incident artifact from the bounded store.
// Missing evidence is reported as unknown; it is never treated as mismatch.
func Write(w http.ResponseWriter, rows []model.Connection) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="httpfin-report.txt"`)
	fmt.Fprintln(w, "httpfin CLIENT FINGERPRINT INSPECTOR REPORT")
	fmt.Fprintln(w, "Generated:", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintln(w, "Evidence policy: missing fields are unknown; fingerprints are signals, not identity proof")
	fmt.Fprintf(w, "Observations: %d\n", len(rows))
	fmt.Fprintln(w, strings.Repeat("=", 118))

	writeSummary(w, rows)
	writeClients(w, rows)
	fmt.Fprintln(w, "\nACCESS SUMMARY")
	fmt.Fprintln(w, strings.Repeat("-", 118))
	fmt.Fprintf(w, "%-20s %-6s %-28s %-3s %-8s %-34s %-16s %-12s\n", "TIME", "MS", "CLIENT", "ST", "BYTES", "REQUEST", "PROFILE", "RISK")
	for _, c := range rows {
		request := c.Method + " " + redactURL(c.URL)
		fmt.Fprintf(w, "%-20s %-6d %-28s %-3d %-8d %-34s %-16s %-12s\n",
			c.Timestamp.Format("2006-01-02 15:04:05"), c.ElapsedMS,
			fingerprint.Truncate(fingerprint.Dash(c.ClientIP), 28), c.Status, c.Bytes,
			fingerprint.Truncate(request, 34), fingerprint.Truncate(fingerprint.Dash(c.DetectedProfile), 16),
			fingerprint.Truncate(fingerprint.Dash(c.Risk), 12))
	}

	fmt.Fprintln(w, "\nOBSERVATION DETAILS")
	fmt.Fprintln(w, strings.Repeat("-", 118))
	for index, c := range rows {
		writeObservation(w, c, previousForClient(rows, index))
	}
}

func writeSummary(w interface{ Write([]byte) (int, error) }, rows []model.Connection) {
	status := make(map[int]int)
	risk := make(map[string]int)
	unknownTLS, unknownJA3, unknownH2 := 0, 0, 0
	for _, c := range rows {
		status[c.Status]++
		risk[fingerprint.Dash(c.Risk)]++
		if c.TLSVersion == "" {
			unknownTLS++
		}
		if c.JA3 == "" {
			unknownJA3++
		}
		if c.HTTP2Fingerprint == "" {
			unknownH2++
		}
	}
	fmt.Fprintln(w, "SUMMARY")
	fmt.Fprintf(w, "  statuses: %s\n", formatCounts(status))
	fmt.Fprintf(w, "  risks: %s\n", formatStringCounts(risk))
	fmt.Fprintf(w, "  evidence unknown: tls=%d/%d ja3=%d/%d http2=%d/%d\n", unknownTLS, len(rows), unknownJA3, len(rows), unknownH2, len(rows))
}

func writeClients(w interface{ Write([]byte) (int, error) }, rows []model.Connection) {
	type client struct {
		count                   int
		latest                  model.Connection
		suspected, notSuspected int
	}
	clients := make(map[string]*client)
	for _, c := range rows {
		key := fingerprint.Dash(c.ClientIP)
		entry := clients[key]
		if entry == nil {
			entry = &client{latest: c}
			clients[key] = entry
		}
		entry.count++
		if policy.Evaluate(c).Action != "" {
			entry.suspected++
		} else {
			entry.notSuspected++
		}
	}
	keys := make([]string, 0, len(clients))
	for key := range clients {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Fprintln(w, "\nCLIENTS AND BOT SIGNALS")
	fmt.Fprintln(w, strings.Repeat("-", 118))
	fmt.Fprintf(w, "%-28s %-8s %-18s %-16s %-12s %-12s\n", "CLIENT", "REQUESTS", "LATEST PROFILE", "BOT SIGNAL", "SUSPECTED", "NOT SUSPECTED")
	for _, key := range keys {
		c := clients[key]
		bot := "not_suspected"
		if c.suspected > 0 {
			bot = "suspected"
		}
		fmt.Fprintf(w, "%-28s %-8d %-18s %-16s %-12d %-12d\n", fingerprint.Truncate(key, 28), c.count, fingerprint.Truncate(unknown(c.latest.DetectedProfile), 18), bot, c.suspected, c.notSuspected)
	}
	if len(keys) == 0 {
		fmt.Fprintln(w, "No observations yet.")
	}
	fmt.Fprintln(w, "Bot signal is observe-only. not_suspected means insufficient evidence, not proof of a human client.")
}

func writeObservation(w interface{ Write([]byte) (int, error) }, c model.Connection, previous *model.Connection) {
	fmt.Fprintf(w, "[%s] id=%s client=%s request=%s status=%d bytes=%d duration_ms=%d\n",
		c.Timestamp.Format(time.RFC3339), fingerprint.Dash(c.ID), fingerprint.Dash(c.ClientIP),
		redactURL(c.Method+" "+c.URL), c.Status, c.Bytes, c.ElapsedMS)
	fmt.Fprintf(w, "  observation: point=%s evidence=%s\n", unknown(c.ObservationPoint), evidenceState(c))
	fmt.Fprintf(w, "  transport: protocol=%s tls=%s sni=%s alpn=%s offered_alpn=%s\n",
		unknown(c.Protocol), unknown(c.TLSVersion), unknown(c.SNI), unknown(c.ALPN), unknown(c.ALPNOffered))
	fmt.Fprintf(w, "  fingerprints: client_hello=%s ja3=%s ja4=%s http2=%s\n",
		unknown(c.ClientHelloSig), unknown(c.JA3), unknown(c.JA4), unknown(c.HTTP2Fingerprint))
	fmt.Fprintf(w, "  declared: user_agent=%s profile=%s\n",
		unknown(fingerprint.Truncate(c.UserAgent, 96)), unknown(c.DetectedProfile))
	decision := policy.Evaluate(c)
	botStatus, botReason := "not_suspected", "insufficient independent signals"
	if decision.Action != "" {
		botStatus, botReason = "suspected", decision.Reason
	}
	fmt.Fprintf(w, "  bot_signal: status=%s action=%s confidence=%.2f reason=%s\n", botStatus, unknown(decision.Action), decision.Confidence, botReason)
	fmt.Fprintf(w, "  fingerprint_change: %s\n", fingerprintChanges(c, previous))
	fmt.Fprintf(w, "  comparison: status=%s reference=%s/%s source=%s explanation=%s\n",
		comparisonStatus(c), unknown(c.ReferenceProfile), unknown(c.ReferenceVersion), unknown(c.ReferenceSource), explanation(c))
}

func evidenceState(c model.Connection) string {
	if c.ClientHelloSig != "" || c.JA3 != "" || c.JA4 != "" || c.HTTP2Fingerprint != "" {
		return "partial-wire"
	}
	if c.TLSVersion != "" || c.SNI != "" || c.ALPN != "" {
		return "transport-only"
	}
	return "http-only"
}

func previousForClient(rows []model.Connection, index int) *model.Connection {
	for i := index + 1; i < len(rows); i++ {
		if rows[i].ClientIP == rows[index].ClientIP {
			return &rows[i]
		}
	}
	return nil
}

func fingerprintChanges(current model.Connection, previous *model.Connection) string {
	if previous == nil {
		return "no_previous_observation"
	}
	changes := make([]string, 0, 6)
	compare := func(name, now, before string) {
		if now != before {
			changes = append(changes, fmt.Sprintf("%s:%s->%s", name, unknown(before), unknown(now)))
		}
	}
	compare("tls", current.TLSVersion, previous.TLSVersion)
	compare("alpn", current.ALPN, previous.ALPN)
	compare("client_hello", current.ClientHelloSig, previous.ClientHelloSig)
	compare("ja3", current.JA3, previous.JA3)
	compare("ja4", current.JA4, previous.JA4)
	compare("http2", current.HTTP2Fingerprint, previous.HTTP2Fingerprint)
	if len(changes) == 0 {
		return "unchanged"
	}
	return strings.Join(changes, ", ")
}

func comparisonStatus(c model.Connection) string {
	if c.BaselineStatus != "" {
		return c.BaselineStatus
	}
	if c.ClientHelloSig == "" {
		return "unavailable"
	}
	return "unknown"
}

func explanation(c model.Connection) string {
	switch c.BaselineStatus {
	case "matched":
		return "observed signature agrees with the selected baseline"
	case "mismatch":
		return "declared profile differs from the known baseline; review with endpoint and behavior context"
	case "unknown":
		return "signature was observed but is not present in the selected baseline"
	default:
		return "insufficient wire-level evidence for a baseline comparison"
	}
}

func unknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return fingerprint.Truncate(value, 256)
}

func formatCounts(values map[int]int) string {
	if len(values) == 0 {
		return "none"
	}
	ordered := []int{200, 201, 204, 301, 302, 400, 401, 403, 404, 429, 500, 502}
	parts := make([]string, 0, len(values))
	for _, key := range ordered {
		if values[key] > 0 {
			parts = append(parts, fmt.Sprintf("%d=%d", key, values[key]))
		}
	}
	remaining := make([]int, 0, len(values))
	for key := range values {
		if !containsInt(ordered, key) {
			remaining = append(remaining, key)
		}
	}
	sort.Ints(remaining)
	for _, key := range remaining {
		if values[key] > 0 {
			parts = append(parts, fmt.Sprintf("%d=%d", key, values[key]))
		}
	}
	return strings.Join(parts, ", ")
}

func formatStringCounts(values map[string]int) string {
	if len(values) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(values))
	for _, key := range keys {
		count := values[key]
		parts = append(parts, fmt.Sprintf("%s=%d", key, count))
	}
	return strings.Join(parts, ", ")
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func redactURL(value string) string {
	if value == "" {
		return "/"
	}
	parts := strings.SplitN(value, " ", 2)
	prefix, target := "", value
	if len(parts) == 2 {
		prefix, target = parts[0]+" ", parts[1]
	}
	u, err := url.Parse(target)
	if err != nil || u.RawQuery == "" {
		return fingerprint.Truncate(prefix+target, 256)
	}
	q := u.Query()
	for key := range q {
		if isSensitiveKey(key) {
			q.Set(key, "[REDACTED]")
		}
	}
	u.RawQuery = q.Encode()
	return fingerprint.Truncate(prefix+u.String(), 256)
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	switch key {
	case "token", "access_token", "refresh_token", "id_token", "api_key", "apikey", "key", "secret", "client_secret", "password", "passwd", "authorization", "cookie", "set_cookie", "session", "session_id":
		return true
	}
	for _, marker := range []string{"_token", "_secret", "_password", "_credential"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return strings.HasPrefix(key, "auth_") || strings.HasPrefix(key, "x_api_")
}
