package report

import (
	"fmt"
	"httpfin/internal/fingerprint"
	"httpfin/internal/model"
	"net/http"
	"strings"
	"time"
)

func Write(w http.ResponseWriter, rows []model.Connection) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="httpfin-report.txt"`)
	fmt.Fprintln(w, "httpfin CLIENT FINGERPRINT INSPECTOR REPORT")
	fmt.Fprintln(w, "Generated:", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintln(w, "Format: Squid-style access summary + passive fingerprint signals")
	fmt.Fprintln(w, strings.Repeat("=", 110))
	fmt.Fprintf(w, "%-20s %-6s %-28s %-3s %-8s %-16s %-14s %-12s\n", "TIME", "MS", "CLIENT", "ST", "BYTES", "METHOD", "PROFILE", "RISK")
	for _, c := range rows {
		fmt.Fprintf(w, "%-20s %-6d %-28s %-3d %-8d %-16s %-14s %-12s\n", c.Timestamp.Format("2006-01-02 15:04:05"), c.ElapsedMS, c.ClientIP, c.Status, c.Bytes, c.Method+" "+fingerprint.Truncate(c.URL, 18), c.DetectedProfile, c.Risk)
	}
	fmt.Fprintln(w, "\nFINGERPRINT DETAILS")
	fmt.Fprintln(w, strings.Repeat("-", 110))
	for _, c := range rows {
		fmt.Fprintf(w, "[%s] id=%s %s | protocol=%s sni=%s tls=%s alpn=%s offered_alpn=%s hello=%s ja3=%s ja4=%s http2=%s ua=%s consistency=%s baseline=%s/%s status=%s\n", c.Timestamp.Format(time.RFC3339), fingerprint.Dash(c.ID), c.ClientIP, fingerprint.Dash(c.Protocol), fingerprint.Dash(c.SNI), fingerprint.Dash(c.TLSVersion), fingerprint.Dash(c.ALPN), fingerprint.Dash(c.ALPNOffered), fingerprint.Dash(c.ClientHelloSig), fingerprint.Dash(c.JA3), fingerprint.Dash(c.JA4), fingerprint.Dash(c.HTTP2Fingerprint), fingerprint.Truncate(c.UserAgent, 64), c.UAConsistency, fingerprint.Dash(c.ReferenceProfile), fingerprint.Dash(c.ReferenceVersion), fingerprint.Dash(c.BaselineStatus))
	}
}
