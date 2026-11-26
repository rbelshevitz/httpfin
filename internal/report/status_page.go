package report

import (
	"bytes"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var statusPageTemplate = template.Must(template.New("status-page").Funcs(template.FuncMap{
	"shortURL": shortURL, "shortUA": shortUA, "shortPoint": shortPoint,
	"transportLabel": transportLabel, "httpStatusClass": httpStatusClass,
	"riskClass": riskClass, "baselineClass": baselineClass,
	"fingerprintCoverage": fingerprintCoverage,
}).Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>httpfin — observations</title><style>*{box-sizing:border-box}body{margin:0;padding:12px 16px;font:12px Arial,Helvetica,sans-serif;color:#20252b;background:#fff}.top{display:flex;justify-content:space-between;border-bottom:1px solid #cfd4d9;padding-bottom:8px;margin-bottom:10px}.brand{font-weight:bold;font-size:14px}.brand span{font-weight:normal;color:#69737d}.live{border:1px solid #aeb6be;padding:2px 6px}.summary{color:#5f6973;margin:0 0 10px}.wrap{overflow:auto;border:1px solid #bfc6cc}table{border-collapse:collapse;width:100%;min-width:1240px}th{background:#fff;color:#4d5862;font-size:10px;text-align:left;text-transform:uppercase}th,td{border-bottom:1px solid #dfe3e6;padding:5px 6px;white-space:nowrap;vertical-align:top}.mono,code{font-family:ui-monospace,SFMono-Regular,Consolas,monospace}.path,.ua{max-width:240px;overflow:hidden;text-overflow:ellipsis}.status.s2{color:#18733d}.status.s3{color:#245da8}.status.s4{color:#9b6800}.status.s5{color:#a42f2f}.profile,.badge{display:inline-block;border:1px solid #c8ced4;border-radius:0;padding:1px 4px;font-size:11px}.badge.high,.badge.mismatch{border-color:#a84a4a;color:#852b2b}.badge.review{border-color:#a87920;color:#745000}.badge.low,.badge.matched{border-color:#518568;color:#2b6543}small{display:block;color:#69737d;font-size:10px;max-width:220px;overflow:hidden;text-overflow:ellipsis}details{white-space:normal}summary{cursor:pointer;color:#59646e}.detail{margin-top:5px;padding:7px;border:1px solid #cfd4d9;min-width:560px;display:grid;grid-template-columns:repeat(4,1fr);gap:5px 12px}.detail h4{grid-column:1/-1;margin:0;font-size:10px;text-transform:uppercase;color:#59646e}.detail b{display:block;font-size:10px;color:#69737d;font-weight:normal}.detail code{font-size:11px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.empty{text-align:center;color:#69737d;padding:16px}</style></head><body><header class="top"><div class="brand">httpfin <span>/ observations</span></div><div class="live" id="mode">{{if .Live}}live{{else}}snapshot{{end}}</div></header><p class="summary" id="summary">{{.Snapshot.Total}} observations · FP coverage {{fingerprintCoverage .Snapshot}} · inspect a row for full evidence</p><div class="wrap"><table><thead><tr><th>Time</th><th>IP</th><th>Method</th><th>Path</th><th>HTTP</th><th>ms</th><th>Profile</th><th>Risk</th><th>Baseline</th><th>TLS / ALPN</th><th>SNI</th><th>UA</th><th>Point</th><th></th></tr></thead><tbody id="rows">{{range .Snapshot.Recent}}<tr><td class="mono" title="{{.Timestamp}}">{{.Timestamp.Format "15:04:05"}}</td><td class="mono">{{.ClientIP}}</td><td class="mono">{{.Method}}</td><td class="path" title="{{.URL}}">{{shortURL .URL}}</td><td class="status {{httpStatusClass .Status}}">{{.Status}}</td><td>{{if .ElapsedMS}}{{.ElapsedMS}}{{else}}—{{end}}</td><td><span class="profile">{{or .DetectedProfile "unknown"}}</span></td><td><span class="badge {{riskClass .Risk}}">{{or .Risk "—"}}</span></td><td><span class="badge {{baselineClass .BaselineStatus}}">{{or .BaselineStatus "—"}}</span></td><td>{{transportLabel .TLSVersion .ALPN}}</td><td>{{or .SNI "—"}}</td><td class="ua" title="{{.UserAgent}}">{{shortUA .UserAgent}}</td><td>{{shortPoint .ObservationPoint}}</td><td><details><summary>inspect</summary><div class="detail"><h4>Access</h4><div><b>id</b><code>{{.ID}}</code></div><div><b>bytes</b><code>{{.Bytes}}</code></div><div><b>protocol</b><code>{{or .Protocol "—"}}</code></div><div><b>time</b><code>{{.Timestamp}}</code></div><h4>Client</h4><div><b>user_agent</b><code>{{or .UserAgent "—"}}</code></div><div><b>profile</b><code>{{or .DetectedProfile "—"}}</code></div><div><b>ua_consistency</b><code>{{or .UAConsistency "—"}}</code></div><h4>TLS</h4><div><b>tls_version</b><code>{{or .TLSVersion "—"}}</code></div><div><b>sni</b><code>{{or .SNI "—"}}</code></div><div><b>alpn</b><code>{{or .ALPN "—"}}</code></div><div><b>offered_alpn</b><code>{{or .ALPNOffered "—"}}</code></div><h4>Fingerprints</h4><div><b>ja3</b><code>{{or .JA3 "not collected"}}</code></div><div><b>ja4</b><code>{{or .JA4 "not collected"}}</code></div><div><b>client_hello</b><code>{{or .ClientHelloSig "not collected"}}</code></div><div><b>http2</b><code>{{or .HTTP2Fingerprint "not collected"}}</code></div><h4>Classification</h4><div><b>risk</b><code>{{or .Risk "—"}}</code></div><div><b>baseline</b><code>{{or .BaselineStatus "—"}}</code></div><div><b>observation_point</b><code>{{or .ObservationPoint "—"}}</code></div></div></details></td></tr>{{else}}<tr><td colspan="14" class="empty">No observations yet.</td></tr>{{end}}</tbody></table></div></body></html>`))

func WriteStatusHTML(w http.ResponseWriter, snapshot Snapshot, live bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if !live {
		w.Header().Set("Content-Disposition", `attachment; filename="httpfin-report.html"`)
	}
	var body bytes.Buffer
	data := struct {
		Snapshot
		Live bool
	}{snapshot, live}
	if err := statusPageTemplate.Execute(&body, data); err != nil {
		return
	}
	html := body.String()
	html = strings.Replace(html, "<th>Point</th>", "<th>Detect</th>", 1)
	html = strings.Replace(html, " observations · FP coverage "+fingerprintCoverage(snapshot)+" · inspect a row for full evidence", " observations · inspect a row for full evidence", 1)
	html = strings.Replace(html, ">unknown</span>", ">—</span>", -1)
	html = strings.Replace(html, "<th>Risk</th>", "", 1)
	for _, row := range snapshot.Recent {
		risk := row.Risk
		if risk == "" {
			risk = "—"
		}
		riskCell := `<td><span class="badge ` + riskClass(row.Risk) + `">` + risk + `</span></td>`
		html = strings.Replace(html, riskCell, "", 1)
		riskDetail := `<div><b>risk</b><code>` + template.HTMLEscapeString(risk) + `</code></div>`
		html = strings.Replace(html, riskDetail, "", 1)
	}
	html = strings.Replace(html, "<th>Baseline</th>", "", 1)
	for _, row := range snapshot.Recent {
		baseline := row.BaselineStatus
		if baseline == "" {
			baseline = "—"
		}
		cell := `<td><span class="badge ` + baselineClass(row.BaselineStatus) + `">` + baseline + `</span></td>`
		html = strings.Replace(html, cell, "", 1)
	}
	html = strings.Replace(html, `colspan="14"`, `colspan="13"`, 1)
	if !snapshot.GeneratedAt.IsZero() {
		html = strings.Replace(html, `httpfin <span>/ observations</span>`, `httpfin <span>/ observations</span> <span class="timezone">time: `+timezoneLabel(snapshot)+`</span>`, 1)
	}
	for _, row := range snapshot.Recent {
		visible := row.Timestamp.Format("15:04:05")
		withDate := row.Timestamp.In(reportLocation()).Format("02 Jan 15:04:05")
		html = strings.Replace(html, ">"+visible+"</td>", ">"+withDate+"</td>", 1)
		rawTime := template.HTMLEscapeString(row.Timestamp.String())
		rawTime = strings.ReplaceAll(rawTime, "+", "&#43;")
		localTime := template.HTMLEscapeString(row.Timestamp.In(reportLocation()).Format("2006-01-02 15:04:05 MST (UTC-07:00)"))
		html = strings.Replace(html, ">"+rawTime+"</code>", ">"+localTime+"</code>", 1)
		transport := transportLabel(row.TLSVersion, row.ALPN)
		if transport != "—" {
			transportHTML := template.HTMLEscapeString(transport)
			html = strings.Replace(html, ">"+transportHTML+"</td>", `><span class="indicator evidence-indicator">`+transportHTML+`</span></td>`, 1)
		}
	}
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		html = strings.Replace(html, ` class="mono">`+method+`</td>`, `><span class="indicator method-indicator">`+method+`</span></td>`, -1)
	}
	for _, row := range snapshot.Recent {
		status := strconv.Itoa(row.Status)
		class := httpStatusClass(row.Status)
		html = strings.Replace(html, `class="status `+class+`">`+status+`</td>`, `class="status `+class+`"><span class="indicator status-indicator">`+status+`</span></td>`, 1)
	}
	html = strings.Replace(html, "</head>", "<style>"+statusPolishCSS+"</style></head>", 1)
	html = strings.Replace(html, `<summary>inspect</summary><div class="detail">`, `<summary>inspect</summary><div class="detail"><button class="detail-close" type="button" aria-label="Close details" onclick="this.closest('details').removeAttribute('open')">close</button>`, -1)
	html = strings.Replace(html, "</body>", statusLegendHTML+"</body>", 1)
	if live {
		html = strings.Replace(html, `</body>`, `<script>`+string(jqueryJS)+`</script><script>`+statusLiveScript+`</script></body>`, 1)
	}
	_, _ = w.Write([]byte(html))
}

func timezoneLabel(snapshot Snapshot) string {
	t := snapshot.GeneratedAt
	if len(snapshot.Recent) > 0 && !snapshot.Recent[0].Timestamp.IsZero() {
		t = snapshot.Recent[0].Timestamp
	}
	return t.In(reportLocation()).Format("MST (UTC-07:00)")
}

func reportLocation() *time.Location {
	return time.Local
}

const statusLiveScript = `(function($){function refresh(){$.getJSON('/api/snapshot',function(s){$('#summary').text(s.total+' observations · inspect a row for full evidence');$('#mode').text('live')}).fail(function(){$('#mode').text('reconnecting')})}refresh();setInterval(refresh,5000);if(window.EventSource){var e=new EventSource('/events');e.addEventListener('observation',refresh)}}(jQuery));`

const statusPolishCSS = `body{font-family:"Segoe UI",Arial,Helvetica,sans-serif;color:#111}.brand span,.summary,summary,.detail h4,.detail b,.legend{color:#111}th{color:#111}.timezone{font-size:11px;font-weight:normal}th,td{vertical-align:middle}.wrap td:last-child{text-align:center}.wrap th:nth-child(3),.wrap td:nth-child(3){text-align:center}.wrap th:nth-child(5),.wrap td:nth-child(5),.wrap th:nth-child(6),.wrap td:nth-child(6){text-align:right}.profile,.badge,.indicator{display:inline-block;min-height:18px;padding:2px 5px;border-radius:0;vertical-align:middle;line-height:13px}.indicator{border:1px solid #555;background:#fff;color:#111;font:11px ui-monospace,SFMono-Regular,Consolas,monospace;line-height:13px}.method-indicator{font-weight:600}.status-indicator{min-width:3ch;text-align:center}.evidence-indicator{border-color:#111;font-weight:600}.detail{position:absolute;right:0;top:20px;z-index:20;width:680px;max-width:calc(100vw - 32px);min-width:0;margin:0;padding:6px;background:#fff;border:1px solid #555;grid-template-columns:repeat(4,minmax(0,1fr));gap:4px 8px;color:#111}.detail>div{min-width:0}.detail code{display:block;color:#111;white-space:normal;overflow-wrap:anywhere;word-break:break-word;line-height:1.2}.detail-close{position:absolute;right:5px;top:4px;padding:0 3px;border:1px solid #555;background:#fff;color:#111;font:11px "Segoe UI",Arial,sans-serif;cursor:pointer}.legend{margin-top:10px;padding:8px 10px;border-top:1px solid #555;font-size:11px;line-height:1.35}.legend b{color:#111}.legend-item{display:block;margin:0 0 5px}.legend-term{display:inline-block;min-width:112px;margin-right:8px;padding:1px 5px;border:1px solid #555;background:#fff;color:#111;font:11px ui-monospace,SFMono-Regular,Consolas,monospace}`

const statusLegendHTML = `<div class="legend"><div class="legend-item"><code class="legend-term">Evidence</code> outlined transport badges are values observed at the collection point. They are stronger than HTTP claims, but are not impossible to imitate.</div><div class="legend-item"><code class="legend-term">Baseline</code> optional comparison of observed <code>ClientHelloSig</code> with the configured baseline. It is shown only in Inspect: <code>matched</code> means signature and detected profile agree; <code>mismatch</code> means they disagree; <code>unknown</code> means the signature was not in the baseline; <code>—</code> means checking was not possible.</div><div class="legend-item"><code class="legend-term">Profile / UA</code> detected profile and client-declared User-Agent recorded for this request. Both are client claims or heuristics and can be changed by the client.</div><div class="legend-item"><code class="legend-term">TLS / ALPN / SNI</code> transport values observed at the collection point. Empty values are expected after TLS termination or for HTTP-only collection.</div><div class="legend-item"><code class="legend-term">Fingerprints</code> JA3, JA4, ClientHello and HTTP/2 fields are shown only when present in the record. Missing means not observed at this collection point.</div><div class="legend-item"><code class="legend-term">Detect</code> recorded collection/detection point: <code>go-http</code>, <code>go-tls</code>, <code>nginx</code> or another source supplied with the observation.</div></div>`
