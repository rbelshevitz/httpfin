package report

import (
	_ "embed"
	"fmt"
	"html/template"
	"httpfin/internal/fingerprint"
	"net/http"
	"strings"
)

// jqueryJS is embedded so dashboard exports and the live UI never depend on a
// CDN or an external network request.
//
//go:embed jquery-3.7.1.min.js
var jqueryJS []byte

//go:embed MaterialSymbolsRounded.woff2
var materialSymbols []byte

var pageTemplate = template.Must(template.New("report").Funcs(template.FuncMap{
	"evidencePercent":     evidencePercent,
	"barHeight":           barHeight,
	"statusClass":         statusClassName,
	"browserClass":        browserClass,
	"browserGlyph":        browserGlyph,
	"browserSymbol":       browserSymbol,
	"browserPercent":      browserPercent,
	"fingerprintCoverage": fingerprintCoverage,
	"httpStatusClass":     httpStatusClass,
	"riskClass":           riskClass,
	"baselineClass":       baselineClass,
	"shortURL":            shortURL,
	"transportLabel":      transportLabel,
	"shortPoint":          shortPoint,
	"shortUA":             shortUA,
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>httpfin — Client behavior investigation</title><style id="compact">body{font-family:"Segoe UI",Inter,ui-sans-serif,system-ui,sans-serif;font-size:13px}.shell{max-width:1360px;padding:18px 24px 34px}.top{margin-bottom:22px}.brand{gap:9px;font-size:16px}.mark{width:30px;height:30px;border-radius:9px}.hero{gap:18px;margin-bottom:16px}.hero h1{font-size:clamp(24px,3vw,36px);margin-bottom:6px}.grid{gap:10px;margin-bottom:12px}.card{border-radius:10px}.kpi{padding:12px 14px}.kpi label{margin-bottom:5px}.kpi strong{font-size:23px}.cols{gap:12px;margin-bottom:12px}.head{padding:12px 14px}h2{font-size:14px}.body{padding:14px}.finding{gap:9px;padding:10px 0}.finding p{margin-top:2px}.bars{height:118px;gap:5px;padding:5px 0 21px}table{font-size:12px}th,td{padding:8px 7px}.pill{padding:3px 6px}.foot{margin-top:12px}</style><style>
:root{color-scheme:dark;--bg:#0b1020;--panel:#121a2d;--panel2:#18233b;--line:#263450;--text:#edf3ff;--muted:#94a4bf;--blue:#79a7ff;--green:#42d39b;--amber:#f4bd63;--red:#ff7474}*{box-sizing:border-box}body{margin:0;background:radial-gradient(circle at 70% -20%,#21345b 0,#0b1020 45%);color:var(--text);font:14px/1.5 Inter,ui-sans-serif,system-ui,sans-serif}.shell{max-width:1440px;margin:auto;padding:28px 34px 56px}.top{display:flex;justify-content:space-between;align-items:center;margin-bottom:34px}.brand{display:flex;gap:12px;align-items:center;font-size:18px;font-weight:700}.mark{display:grid;place-items:center;width:34px;height:34px;border-radius:11px;background:linear-gradient(135deg,#73a3ff,#5c67d8);box-shadow:0 8px 24px #546fe055}.sub,.muted{color:var(--muted)}.live{border:1px solid #2e5b59;background:#12302c;color:var(--green);border-radius:999px;padding:7px 12px;font-size:12px}.hero{display:flex;justify-content:space-between;gap:24px;align-items:end;margin-bottom:24px}.eyebrow{color:var(--blue);font-size:11px;font-weight:700;letter-spacing:.14em;text-transform:uppercase;margin:0 0 8px}.hero h1{font-size:clamp(27px,4vw,44px);letter-spacing:-.045em;line-height:1.05;margin:0 0 10px}.hero p{margin:0;color:var(--muted);max-width:680px}.state{min-width:175px;text-align:right}.state b{font-size:20px}.state small{display:block;color:var(--muted);margin-top:4px}.grid{display:grid;grid-template-columns:repeat(4,1fr);gap:14px;margin-bottom:18px}.card{background:linear-gradient(145deg,#151f35dd,#10182add);border:1px solid var(--line);border-radius:16px;box-shadow:0 16px 45px #00000022}.kpi{padding:18px 19px}.kpi label{display:block;color:var(--muted);font-size:12px;margin-bottom:10px}.kpi strong{font-size:27px;letter-spacing:-.03em}.kpi span{display:block;color:var(--muted);font-size:12px;margin-top:4px}.cols{display:grid;grid-template-columns:1.25fr .75fr;gap:18px;margin-bottom:18px}.head{display:flex;justify-content:space-between;align-items:center;padding:18px 20px;border-bottom:1px solid var(--line)}h2{font-size:15px;margin:0}.body{padding:20px}.finding{display:flex;gap:12px;padding:14px 0;border-bottom:1px solid #26345099}.finding:last-child{border:0}.sev{font-size:10px;font-weight:800;letter-spacing:.08em;padding:4px 7px;border-radius:6px;height:max-content}.sev.high{color:#ffaaaa;background:#572737}.sev.medium{color:#ffd28c;background:#55401e}.finding b{display:block}.finding p{color:var(--muted);font-size:12px;margin:3px 0 0}.bars{display:flex;align-items:end;gap:8px;height:145px;padding:8px 0 24px;border-bottom:1px solid var(--line)}.bar{flex:1;min-width:8px;border-radius:5px 5px 0 0;background:linear-gradient(#79a7ff,#405ba5);position:relative}.bar i{position:absolute;bottom:-23px;left:50%;font-style:normal;transform:translateX(-50%);font-size:10px;color:var(--muted)}table{width:100%;border-collapse:collapse;font-size:13px}th{text-align:left;color:var(--muted);font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.06em}th,td{padding:12px 10px;border-bottom:1px solid #26345099}td:first-child,th:first-child{padding-left:0}td:last-child,th:last-child{padding-right:0}.pill{display:inline-block;padding:4px 8px;border-radius:999px;background:#203457;color:#a9c5ff;font-size:11px}.empty{padding:25px 0;color:var(--muted)}.foot{color:var(--muted);font-size:12px;margin-top:20px}@media(max-width:900px){.shell{padding:22px 18px}.grid{grid-template-columns:repeat(2,1fr)}.cols{grid-template-columns:1fr}.hero{align-items:start;flex-direction:column}.state{text-align:left}}@media(max-width:520px){.grid{grid-template-columns:1fr}.shell{padding:18px 13px}table{font-size:12px}th:nth-child(3),td:nth-child(3){display:none}}
</style></head><body><main class="shell"><header class="top"><div class="brand"><span class="mark">✣</span>httpfin <span class="sub">/ client traffic map</span></div><div class="live">● <span id="mode">Snapshot</span></div></header>
<section class="hero"><div><p class="eyebrow">Client observations</p><h1 id="status" class="{{statusClass .Status}}">{{.Status}}</h1><p id="summary">{{if .Total}}{{.Total}} raw observations from {{.WindowFrom}} to {{.WindowTo}}. Select a connection to inspect its transport and fingerprint signals.{{else}}No observations yet. Start traffic to build an evidence window.{{end}}</p></div><div class="state"><b id="findingCount">{{len .Findings}} findings</b><small>Generated {{.GeneratedAt}}</small></div></section>
<section class="grid"><div class="card kpi"><label>Observations</label><strong id="total">{{.Total}}</strong><span>bounded in-memory window</span></div><div class="card kpi"><label>Sources</label><strong id="sources">{{len .Sources}}</strong><span>source keys, not identities</span></div><div class="card kpi"><label>User-Agent families</label><strong id="userAgents">{{len .UserAgents}}</strong><span>declared client variants</span></div><div class="card kpi"><label>Destinations</label><strong id="destinations">{{len .Destinations}}</strong><span>redacted request targets</span></div></section>
<section class="card browser-card browser-summary"><div class="head"><h2>Browser share</h2><span class="muted">% of all requests</span></div><div class="browser-grid" id="browserSummary">{{range .Browsers}}<div class="browser-item"><span class="material-symbols browser-icon {{browserClass .Family}}">{{browserSymbol .Family}}</span><div><b>{{.Family}} <strong>{{browserPercent .Count $.Total}}</strong></b><small>{{.Count}} requests</small></div></div>{{end}}</div></section>
<section class="card fingerprint-card"><div class="head"><h2>Client observations</h2><span class="muted">Recent requests · transport and client signals · FP coverage <span id="fpCoverage">{{fingerprintCoverage .Snapshot}}</span></span></div><div class="fingerprint-table"><table><thead><tr><th>Time</th><th>IP</th><th>Method</th><th>Path</th><th>HTTP</th><th>MS</th><th>Profile</th><th>Risk</th><th>Baseline</th><th>TLS / ALPN</th><th>SNI</th><th>UA</th><th>Point</th></tr></thead><tbody id="fingerprints">{{range .Recent}}<tr><td class="mono" title="{{.Timestamp}}">{{.Timestamp.Format "15:04:05"}}</td><td class="mono">{{.ClientIP}}</td><td class="mono">{{.Method}}</td><td title="{{.URL}}">{{shortURL .URL}}</td><td class="status {{httpStatusClass .Status}}">{{.Status}}</td><td>{{if .ElapsedMS}}{{.ElapsedMS}}{{else}}—{{end}}</td><td><span class="profile">{{or .DetectedProfile "Unknown"}}</span></td><td><span class="badge {{riskClass .Risk}}">{{or .Risk "—"}}</span></td><td><span class="badge {{baselineClass .BaselineStatus}}">{{or .BaselineStatus "—"}}</span></td><td>{{transportLabel .TLSVersion .ALPN}}</td><td title="{{.SNI}}">{{or .SNI "—"}}</td><td title="{{.UserAgent}}">{{shortUA .UserAgent}}</td><td>{{shortPoint .ObservationPoint}}</td></tr>{{else}}<tr><td colspan="13" class="empty">No client observations yet.</td></tr>{{end}}</tbody></table></div></section>
<section class="card route-card"><div class="head"><h2>Who connects → where they go</h2><span class="muted">Top observed paths</span></div><div class="route-list" id="routes">{{range .Routes}}<div class="route-row"><div class="route-source"><b>{{.Source}}</b><small>{{or .Profile "Unknown"}}</small></div><span class="route-client">{{or .UserAgent "Unknown UA"}}</span><span class="route-arrow" aria-hidden="true">→</span><div class="route-destination"><b>{{.Destination}}</b><small>{{.Count}} requests{{if .Errors}} · {{.Errors}} errors{{end}}</small></div></div>{{else}}<div class="empty">No routes observed yet.</div>{{end}}</div></section>
<section class="cols"><section class="card"><div class="head"><h2>Response status</h2><span class="muted" id="refresh">{{if .Live}}Auto-refresh 5s{{else}}Static report{{end}}</span></div><div class="body"><div class="bars" id="bars">{{range .Statuses}}<div class="bar" style="height:{{barHeight .Count $.Total}}%"><i>{{.Name}}</i></div>{{end}}</div></div></section><section class="card"><div class="head"><h2>Needs attention</h2><span class="muted">{{len .Findings}}</span></div><div class="body" id="findings">{{if .Findings}}{{range .Findings}}<article class="finding"><span class="sev {{.Severity}}">{{.Severity}}</span><div><b>{{.Title}}</b><p>{{.Explanation}} · {{.Source}}</p></div></article>{{end}}{{else}}<div class="empty">No explainable findings in this window.</div>{{end}}</div></section></section>
<section class="card browser-card"><div class="head"><h2>Browser mix</h2><span class="muted">Share of observed requests</span></div><div class="browser-grid" id="browsers">{{range .Browsers}}<div class="browser-item"><span class="material-symbols browser-icon {{browserClass .Family}}">{{browserSymbol .Family}}</span><div><b>{{.Family}} <strong>{{browserPercent .Count $.Total}}</strong></b><small>{{.Count}} requests · {{.SourceCount}} sources{{if .Errors}} · {{.Errors}} errors{{end}}</small></div></div>{{else}}<div class="empty">No browser families observed yet.</div>{{end}}</div></section>
<section class="cols"><section class="card"><div class="head"><h2>Who is arriving</h2><span class="muted">Sources and User-Agents</span></div><div class="body"><table><thead><tr><th>Source</th><th>User-Agent</th><th>Profile</th><th>Requests</th></tr></thead><tbody id="sourcesTable">{{range .Sources}}<tr><td>{{.Source}}</td><td>{{or .UserAgent "Unknown UA"}}</td><td><span class="pill">{{or .Profile "Unknown"}}</span></td><td>{{.Count}}</td></tr>{{else}}<tr><td colspan="4" class="empty">No sources observed yet.</td></tr>{{end}}</tbody></table></div></section><section class="card"><div class="head"><h2>Where they go</h2><span class="muted">Destinations</span></div><div class="body"><table><thead><tr><th>Destination</th><th>Requests</th><th>Sources</th><th>Errors</th></tr></thead><tbody id="destinationsTable">{{range .Destinations}}<tr><td>{{.Destination}}</td><td>{{.Count}}</td><td>{{.SourceCount}}</td><td>{{.Errors}}</td></tr>{{else}}<tr><td colspan="4" class="empty">No destinations observed yet.</td></tr>{{end}}</tbody></table></div></section></section>
<section class="card"><div class="head"><h2>User-Agent families</h2><span class="muted">Declared client identity and reach</span></div><div class="body"><table><thead><tr><th>User-Agent</th><th>Profile</th><th>Requests</th><th>Sources</th><th>Errors</th></tr></thead><tbody id="userAgentsTable">{{range .UserAgents}}<tr><td>{{.UserAgent}}</td><td>{{or .Profile "Unknown"}}</td><td>{{.Count}}</td><td>{{.SourceCount}}</td><td>{{.Errors}}</td></tr>{{else}}<tr><td colspan="5" class="empty">No User-Agent values observed yet.</td></tr>{{end}}</tbody></table></div></section>
<p class="foot">Fingerprints are signals, not identity proof. Missing evidence is unknown, not a mismatch. Report generated from the current in-memory observation window.</p></main>
{{if .Live}}<script>const esc=s=>String(s??'').replace(/[&<>"']/g,m=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'}[m]));async function refresh(){try{const r=await fetch('/api/snapshot',{cache:'no-store'});const s=await r.json();document.querySelector('#status').textContent=s.status;document.querySelector('#total').textContent=s.total;document.querySelector('#errors').textContent=s.errors;document.querySelector('#sources').textContent=s.sources.length;document.querySelector('#findingCount').textContent=s.findings.length+' findings';document.querySelector('#evidence').textContent=Math.round(100*(s.evidence.tls+s.evidence.alpn+s.evidence.client_hello+s.evidence.ja3+s.evidence.ja4+s.evidence.http2)/(Math.max(1,s.total)*6))+'%';document.querySelector('#findings').innerHTML=s.findings.length?s.findings.map(f=>'<article class="finding"><span class="sev '+esc(f.severity)+'">'+esc(f.severity)+'</span><div><b>'+esc(f.title)+'</b><p>'+esc(f.explanation)+' · '+esc(f.source)+'</p></div></article>').join(''):'<div class="empty">No explainable findings in this window.</div>';document.querySelector('#sourcesTable').innerHTML=s.sources.map(x=>'<tr><td>'+esc(x.source)+'</td><td>'+x.count+'</td><td>'+esc(x.profile||'Unknown')+'</td><td><span class="pill">'+esc(x.evidence)+'</span></td><td>'+esc(x.baseline||'—')+'</td></tr>').join('')||'<tr><td colspan="5" class="empty">No sources observed yet.</td></tr>'}catch(e){document.querySelector('#mode').textContent='Disconnected'}}refresh();setInterval(refresh,5000)</script>{{end}}</body></html>`))

func WriteHTML(w http.ResponseWriter, snapshot Snapshot, live bool) {
	WriteStatusHTML(w, snapshot, live)
}

func MaterialSymbolsData() []byte { return materialSymbols }

const polishCSS = `body{font-family:"Segoe UI",Inter,ui-sans-serif,system-ui,sans-serif;font-size:13px;letter-spacing:.005em}.shell{max-width:1360px;padding:18px 24px 34px}.top{margin-bottom:22px}.brand{gap:9px;font-size:16px}.mark{width:30px;height:30px;border-radius:9px}.hero{gap:18px;margin-bottom:16px}.hero h1{font-size:clamp(24px,3vw,36px);margin-bottom:6px}.hero h1.normal{color:var(--green)}.hero h1.review{color:var(--amber)}.hero h1.degraded{color:var(--red)}.hero h1.insufficient{color:var(--muted)}.grid{gap:10px;margin-bottom:12px}.card{border-radius:10px}.kpi{padding:12px 14px}.kpi label{margin-bottom:5px}.kpi strong{font-size:23px}.cols{gap:12px;margin-bottom:12px}.head{padding:12px 14px}.body{padding:14px}.finding{gap:9px;padding:10px 0}.finding p{margin-top:2px}.bars{height:118px;gap:5px;padding:5px 0 21px}table{table-layout:fixed;font-size:12px}th,td{padding:8px 7px;vertical-align:middle}td{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.pill{padding:3px 6px}.foot{margin-top:12px}#sourcesTable th:nth-child(1){width:22%}#sourcesTable th:nth-child(2){width:40%}#sourcesTable th:nth-child(3){width:23%}#sourcesTable th:nth-child(4){width:15%}#destinationsTable th:nth-child(1){width:52%}#destinationsTable th:nth-child(2){width:16%}#destinationsTable th:nth-child(3){width:16%}#destinationsTable th:nth-child(4){width:16%}#userAgentsTable th:nth-child(1){width:48%}#userAgentsTable th:nth-child(2){width:22%}#userAgentsTable th:nth-child(3){width:10%}#userAgentsTable th:nth-child(4){width:10%}#userAgentsTable th:nth-child(5){width:10%}@media(max-width:900px){.shell{padding:16px 18px}.grid{grid-template-columns:repeat(2,1fr)}}@media(max-width:520px){.shell{padding:14px 12px}.grid{grid-template-columns:1fr}table{font-size:11px}}`

const browserPolishCSS = `.browser-card{margin-bottom:12px}.browser-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:8px;padding:10px 14px}.browser-item{display:flex;align-items:center;gap:9px;min-width:0;padding:8px 9px;border:1px solid #26345099;border-radius:8px;background:#10182a88}.browser-item b{display:block;font-size:12px}.browser-item small{display:block;color:var(--muted);font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.browser-icon{display:grid;place-items:center;flex:0 0 27px;width:27px;height:27px;border-radius:8px;color:#fff;font-size:11px;font-weight:800}.browser-icon.chrome{background:linear-gradient(135deg,#4285f4 0 34%,#34a853 34% 62%,#fbbc05 62% 80%,#ea4335 80%)}.browser-icon.edge{background:linear-gradient(135deg,#0bb7a7,#2878e8)}.browser-icon.firefox{background:linear-gradient(135deg,#ff9b2f,#e44326 55%,#7b38d8)}.browser-icon.safari{background:linear-gradient(135deg,#35bff3,#1674d1)}.browser-icon.opera{background:#d52b3e}.browser-icon.curl{background:#303b52}.browser-icon.other{background:#59677f}@media(max-width:520px){.browser-grid{grid-template-columns:repeat(2,1fr)}}`

const browserIconCSS = `@font-face{font-family:"Material Symbols Rounded";src:url("/report/material-symbols.woff2") format("woff2");font-weight:100 700;font-style:normal;font-display:swap}.material-symbols{font-family:"Material Symbols Rounded";font-weight:400;font-style:normal;font-size:20px;line-height:1;letter-spacing:normal;text-transform:none;display:inline-block;white-space:nowrap;word-wrap:normal;direction:ltr;-webkit-font-feature-settings:"liga";-webkit-font-smoothing:antialiased}.browser-icon{background:#f7f9fc;color:#345b9d}.browser-icon.chrome{color:#4285f4}.browser-icon.edge{color:#1686c9}.browser-icon.firefox{color:#e65b2e}.browser-icon.safari{color:#1674d1}.browser-icon.opera{color:#d52b3e}.browser-icon.curl{color:#59677f}.browser-icon.other{color:#59677f}`

const browserLayoutCSS = `.browser-item>div{min-width:0;overflow:hidden}.browser-item small{max-width:100%}`

const routeCSS = `.route-card{margin-bottom:12px}.route-list{padding:6px 14px 10px}.route-row{display:grid;grid-template-columns:minmax(110px,.8fr) minmax(150px,1.1fr) 24px minmax(180px,1.35fr);align-items:center;gap:10px;min-height:52px;border-bottom:1px solid #edf0f5}.route-row:last-child{border-bottom:0}.route-row b,.route-row small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.route-row b{font-size:12px}.route-row small{color:var(--muted);font-size:11px;margin-top:2px}.route-source,.route-destination,.route-client{min-width:0;overflow:hidden}.route-client{font-size:11px;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.route-arrow{display:grid;place-items:center;width:24px;height:24px;border-radius:50%;background:#edf3ff;color:var(--blue);font-size:16px;font-weight:700}.route-destination{padding-left:10px;border-left:2px solid #dbe6fb}@media(max-width:700px){.route-row{grid-template-columns:minmax(100px,1fr) 22px minmax(140px,1.2fr)}.route-client{grid-column:1;grid-row:2;font-size:10px;margin-top:-13px}.route-arrow{grid-column:2;grid-row:1 / 3}.route-destination{grid-column:3;grid-row:1 / 3}}`

const typographyCSS = `body,.shell{font-family:Arial,Helvetica,sans-serif}.brand,.hero h1,h2,th,td{font-family:Arial,Helvetica,sans-serif}`

const fingerprintCSS = `.fingerprint-card{margin-bottom:12px}.fingerprint-table{overflow:auto;padding:0 14px 8px}.fingerprint-table table{min-width:980px}.fingerprint-table th,.fingerprint-table td{padding:8px 7px}.fingerprint-table td:first-child{min-width:230px}.fingerprint-table td small{display:block;color:var(--muted);max-width:280px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.mono{font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:11px;max-width:150px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}`

const instrumentCSS = `body{background:#fff;color:#1d232b;font-family:Arial,Helvetica,sans-serif}.shell{max-width:1500px;padding:14px 18px 28px}.top{margin-bottom:12px;border-bottom:1px solid #d7dbe0;padding-bottom:9px}.brand{font-size:14px}.mark{width:20px;height:20px;border-radius:2px;background:#1d232b;color:#fff}.sub{font-weight:400}.live{border-radius:2px;padding:3px 7px;background:#fff;border:1px solid #aeb5bd;color:#303840}.hero{margin-bottom:10px;gap:12px;display:block}.hero #status,.hero .state{display:none}.hero h1{font-size:18px;letter-spacing:0;color:#1d232b;margin:0 0 2px}.eyebrow{font-size:10px;color:#606a74;margin-bottom:3px}.hero p{font-size:12px}.card{border:1px solid #cbd1d7;border-radius:2px;box-shadow:none;background:#fff}.grid{gap:0;border:1px solid #cbd1d7;margin-bottom:10px}.grid .card{border:0;border-right:1px solid #d7dbe0;border-radius:0}.grid .card:last-child{border-right:0}.kpi{padding:8px 10px}.kpi strong{font-size:18px}.kpi label,.kpi span{font-size:11px}.browser-card,.route-card{display:none}.fingerprint-card{margin-bottom:10px}.head{padding:8px 10px;background:#f2f4f6;border-bottom:1px solid #cbd1d7}.head h2{font-size:12px;font-weight:700}.muted{font-size:11px}.fingerprint-table{padding:0 8px 4px}.fingerprint-table table{min-width:100%}.fingerprint-table th,.fingerprint-table td{padding:6px 6px;border-bottom:1px solid #e1e4e7}.fingerprint-table th{font-size:10px;color:#4e5862}.fingerprint-table td{font-size:12px}.mono{font-size:11px}.cols{gap:10px;margin-bottom:10px}.cols:has(#findings){grid-template-columns:1fr}.cols:has(#findings) .card:has(#findings){display:none}.body{padding:9px}.bars{height:82px}.finding{padding:7px 0}.foot{margin-top:8px;font-size:11px}`

const hideLegacyCSS = `.cols:has(#findings),.cols:has(#bars){display:none}`

const noFillCSS = `body,.card,.head,.grid,.grid .card,.live,.pill{background:#fff;background-image:none;box-shadow:none}.mark{background:#fff;border:1px solid #1d232b;color:#1d232b}.head{background:#fff}.live{border-radius:0}.bar{background:#1d232b}`

const statusTableCSS = `.fingerprint-table{overflow-x:auto}.fingerprint-table table{min-width:1180px;table-layout:auto}.fingerprint-table th,.fingerprint-table td{white-space:nowrap}.fingerprint-table th:first-child,.fingerprint-table td:first-child{position:sticky;left:0;background:#fff;z-index:1}.fingerprint-table th:nth-child(2),.fingerprint-table td:nth-child(2){position:sticky;left:68px;background:#fff;z-index:1}.fingerprint-table td:nth-child(4){max-width:260px;overflow:hidden;text-overflow:ellipsis}.fingerprint-table td:nth-child(12){max-width:280px;overflow:hidden;text-overflow:ellipsis}.status.s2{color:#18733d}.status.s3{color:#245da8}.status.s4{color:#9b6800}.status.s5{color:#a42f2f}.badge,.profile{display:inline-block;padding:1px 4px;border:1px solid #cbd1d7;border-radius:2px;font-size:11px}.badge.high,.badge.mismatch{border-color:#b34a4a;color:#8d2525}.badge.review{border-color:#b78324;color:#805800}.badge.low,.badge.matched{border-color:#4f9168;color:#28633e}.badge.info,.badge.unknown{color:#58636e}`

const flatCSS = `:root{color-scheme:light;--bg:#f4f6fa;--panel:#fff;--panel2:#f8fafc;--line:#dfe5ee;--text:#172033;--muted:#6d7a8f;--blue:#3768d4;--green:#16875f;--amber:#a86608;--red:#c33d4b}body{background:#f4f6fa;color:var(--text)}.card{background:#fff;border-color:var(--line);box-shadow:0 2px 10px rgba(28,43,70,.06)}.mark{background:#3768d4;box-shadow:none}.live{border-color:#bde7d7;background:#effaf5;color:#16875f}.browser-item{background:#f8fafc;border-color:var(--line)}.browser-icon{background:#fff}.bar{background:#6b8fdc}.finding{border-color:#edf0f5}th,td{border-color:#edf0f5}.pill{background:#edf3ff;color:#3768d4}.foot{color:#7d8999}`

func browserClass(family string) string {
	switch strings.ToLower(family) {
	case "chrome", "android · chrome":
		return "chrome"
	case "edge", "android · edge":
		return "edge"
	case "firefox", "android · firefox":
		return "firefox"
	case "safari", "ios · safari":
		return "safari"
	case "opera", "android · opera":
		return "opera"
	case "curl":
		return "curl"
	default:
		return "other"
	}
}

func browserGlyph(family string) string {
	value := strings.ToLower(family)
	if strings.HasPrefix(value, "android") {
		return "A"
	}
	if strings.HasPrefix(value, "ios") {
		return "i"
	}
	switch {
	case strings.Contains(value, "chrome"):
		return "C"
	case strings.Contains(value, "edge"):
		return "E"
	case strings.Contains(value, "firefox"):
		return "F"
	case strings.Contains(value, "safari"):
		return "S"
	case strings.Contains(value, "opera"):
		return "O"
	case strings.Contains(value, "curl"):
		return "⌁"
	default:
		return "?"
	}
}

func browserSymbol(family string) string {
	value := strings.ToLower(family)
	if strings.HasPrefix(value, "android") {
		return "android"
	}
	if strings.HasPrefix(value, "ios") {
		return "phone_iphone"
	}
	switch {
	case strings.Contains(value, "safari"):
		return "language"
	case strings.Contains(value, "chrome"), strings.Contains(value, "edge"), strings.Contains(value, "firefox"), strings.Contains(value, "opera"):
		return "public"
	default:
		return "devices"
	}
}

const liveScript = `
(function () {
  function esc(s) { return String(s == null ? '' : s).replace(/[&<>"']/g, function (m) { return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'}[m]); }); }
  function evidence(s) {
    if (!s.total) return '—';
    var e = s.evidence, n = e.tls + e.alpn + e.client_hello + e.ja3 + e.ja4 + e.http2;
    return Math.round(n / (s.total * 6) * 100) + '%';
  }
  function render(s) {
    var status = document.querySelector('#status');
    status.textContent = s.status;
    status.className = ({'Normal':'normal','Review':'review','Degraded':'degraded','Insufficient evidence':'insufficient'}[s.status] || 'insufficient');
    document.querySelector('#total').textContent = s.total;
    document.querySelector('#sources').textContent = s.sources.length;
    document.querySelector('#userAgents').textContent = s.user_agents.length;
    document.querySelector('#destinations').textContent = s.destinations.length;
    document.querySelector('#evidence').textContent = evidence(s);
    document.querySelector('#findingCount').textContent = s.findings.length + ' findings';
    document.querySelector('#summary').textContent = s.total ? s.total + ' observations from ' + s.window_from + ' to ' + s.window_to + '.' : 'No observations yet. Start traffic to build an evidence window.';
    var max = s.total || 1;
    document.querySelector('#bars').innerHTML = s.statuses.map(function (x) { return '<div class="bar" style="height:' + Math.max(8, Math.round(x.count / max * 100)) + '%"><i>' + esc(x.name) + '</i></div>'; }).join('');
    document.querySelector('#findings').innerHTML = s.findings.length ? s.findings.map(function (f) { return '<article class="finding"><span class="sev ' + esc(f.severity) + '">' + esc(f.severity) + '</span><div><b>' + esc(f.title) + '</b><p>' + esc(f.explanation) + ' · ' + esc(f.source) + '</p></div></article>'; }).join('') : '<div class="empty">No explainable findings in this window.</div>';
    document.querySelector('#sourcesTable').innerHTML = s.sources.map(function (x) { return '<tr><td>' + esc(x.source) + '</td><td>' + esc(x.user_agent || 'Unknown UA') + '</td><td><span class="pill">' + esc(x.profile || 'Unknown') + '</span></td><td>' + x.count + '</td></tr>'; }).join('') || '<tr><td colspan="4" class="empty">No sources observed yet.</td></tr>';
    document.querySelector('#destinationsTable').innerHTML = s.destinations.map(function (x) { return '<tr><td>' + esc(x.destination) + '</td><td>' + x.count + '</td><td>' + x.source_count + '</td><td>' + x.errors + '</td></tr>'; }).join('') || '<tr><td colspan="4" class="empty">No destinations observed yet.</td></tr>';
    document.querySelector('#userAgentsTable').innerHTML = s.user_agents.map(function (x) { return '<tr><td>' + esc(x.user_agent) + '</td><td>' + esc(x.profile || 'Unknown') + '</td><td>' + x.count + '</td><td>' + x.source_count + '</td><td>' + x.errors + '</td></tr>'; }).join('') || '<tr><td colspan="5" class="empty">No User-Agent values observed yet.</td></tr>';
  }
  async function refresh() {
    try {
      var response = await fetch('/api/snapshot', {cache: 'no-store'});
      render(await response.json());
      document.querySelector('#mode').textContent = 'Live';
    } catch (_) { document.querySelector('#mode').textContent = 'Reconnecting'; }
  }
  refresh();
  setInterval(refresh, 5000);
  if (window.EventSource) {
    var source = new EventSource('/events');
    source.addEventListener('observation', refresh);
    source.addEventListener('ready', function () { document.querySelector('#mode').textContent = 'Live'; });
    source.onerror = function () { document.querySelector('#mode').textContent = 'Reconnecting'; };
  }
}());`

const jqueryLiveScript = `(function ($) {
  function esc(value) { return $('<div>').text(value == null ? '' : value).html(); }
  function evidence(s) {
    if (!s.total) return '—';
    var e = s.evidence, n = e.tls + e.alpn + e.client_hello + e.ja3 + e.ja4 + e.http2;
    return Math.round(n / (s.total * 6) * 100) + '%';
  }
  function render(s) {
    var status = ({Normal:'normal',Review:'review',Degraded:'degraded','Insufficient evidence':'insufficient'}[s.status] || 'insufficient');
    $('#status').text(s.status).attr('class', status);
    $('#total').text(s.total);
    $('#sources').text(s.sources.length);
    $('#userAgents').text(s.user_agents.length);
    $('#destinations').text(s.destinations.length);
    $('#fpCoverage').text(s.total ? Math.round((s.evidence.ja3 + s.evidence.ja4 + s.evidence.http2) / (s.total * 3) * 100) + '%' : '—');
    $('#evidence').text(evidence(s));
    $('#findingCount').text(s.findings.length + ' findings');
    $('#summary').text(s.total ? s.total + ' observations from ' + s.window_from + ' to ' + s.window_to + '. Follow the routes below to see who connected and where they went.' : 'No observations yet. Start traffic to build an evidence window.');
    var max = s.total || 1;
    $('#bars').html($.map(s.statuses, function (x) { return '<div class="bar" style="height:' + Math.max(8, Math.round(x.count / max * 100)) + '%"><i>' + esc(x.name) + '</i></div>'; }).join(''));
    $('#findings').html(s.findings.length ? $.map(s.findings, function (f) { return '<article class="finding"><span class="sev ' + esc(f.severity) + '">' + esc(f.severity) + '</span><div><b>' + esc(f.title) + '</b><p>' + esc(f.explanation) + ' · ' + esc(f.source) + '</p></div></article>'; }).join('') : '<div class="empty">No explainable findings in this window.</div>');
    $('#routes').html($.map(s.routes, function (x) { return '<div class="route-row"><div class="route-source"><b>' + esc(x.source) + '</b><small>' + esc(x.profile || 'Unknown') + '</small></div><span class="route-client">' + esc(x.user_agent || 'Unknown UA') + '</span><span class="route-arrow" aria-hidden="true">→</span><div class="route-destination"><b>' + esc(x.destination) + '</b><small>' + x.count + ' requests' + (x.errors ? ' · ' + x.errors + ' errors' : '') + '</small></div></div>'; }).join('') || '<div class="empty">No routes observed yet.</div>');
    $('#sourcesTable').html($.map(s.sources, function (x) { return '<tr><td>' + esc(x.source) + '</td><td>' + esc(x.user_agent || 'Unknown UA') + '</td><td><span class="pill">' + esc(x.profile || 'Unknown') + '</span></td><td>' + x.count + '</td></tr>'; }).join('') || '<tr><td colspan="4" class="empty">No sources observed yet.</td></tr>');
    $('#destinationsTable').html($.map(s.destinations, function (x) { return '<tr><td>' + esc(x.destination) + '</td><td>' + x.count + '</td><td>' + x.source_count + '</td><td>' + x.errors + '</td></tr>'; }).join('') || '<tr><td colspan="4" class="empty">No destinations observed yet.</td></tr>');
    $('#userAgentsTable').html($.map(s.user_agents, function (x) { return '<tr><td>' + esc(x.user_agent) + '</td><td>' + esc(x.profile || 'Unknown') + '</td><td>' + x.count + '</td><td>' + x.source_count + '</td><td>' + x.errors + '</td></tr>'; }).join('') || '<tr><td colspan="5" class="empty">No User-Agent values observed yet.</td></tr>');
    $('#browsers').html($.map(s.browsers, function (x) { var key = x.family.toLowerCase().replace(/[^a-z]+/g, '-'); var v = x.family.toLowerCase(), symbol = v.indexOf('android') === 0 ? 'android' : (v.indexOf('ios') === 0 ? 'phone_iphone' : (v.indexOf('chrome') >= 0 ? 'public' : (v.indexOf('firefox') >= 0 ? 'public' : (v.indexOf('safari') >= 0 ? 'language' : (v.indexOf('opera') >= 0 ? 'public' : 'public'))))); var share = s.total ? Math.round(x.count / s.total * 100) : 0; return '<div class="browser-item"><span class="material-symbols browser-icon ' + key.split('-').pop() + '">' + symbol + '</span><div><b>' + esc(x.family) + ' <strong>' + share + '%</strong></b><small>' + x.count + ' requests · ' + x.source_count + ' sources' + (x.errors ? ' · ' + x.errors + ' errors' : '') + '</small></div></div>'; }).join('') || '<div class="empty">No browser families observed yet.</div>');
    $('#fingerprints').html($.map(s.recent, function (x) { var fp = x.ja3 || x.ja4 || x.http2_fingerprint; var statusClass = x.status >= 500 ? 's5' : (x.status >= 400 ? 's4' : (x.status >= 300 ? 's3' : (x.status >= 200 ? 's2' : 'sx'))); var riskClass = x.risk === 'High' ? 'high' : (x.risk === 'Review' ? 'review' : (x.risk === 'Low' ? 'low' : 'info')); var baseClass = x.baseline_status === 'mismatch' ? 'mismatch' : (x.baseline_status === 'matched' ? 'matched' : 'unknown'); var tls = x.tls_version ? x.tls_version.replace(/^TLS /, '') : '—'; var transport = x.alpn ? tls + ' / ' + x.alpn : tls; var point = x.observation_point === 'nginx-http-module' ? 'nginx' : (x.observation_point || '—'); return '<tr><td class="mono" title="' + esc(x.timestamp || '') + '">' + esc((x.timestamp || '').slice(11,19)) + '</td><td class="mono">' + esc(x.client_ip) + '</td><td class="mono">' + esc(x.method || '—') + '</td><td title="' + esc(x.url || '') + '">' + esc((x.url || '').slice(0,42)) + '</td><td class="status ' + statusClass + '">' + x.status + '</td><td>' + (x.elapsed_ms || '—') + '</td><td><span class="profile">' + esc(x.detected_profile || 'Unknown') + '</span></td><td><span class="badge ' + riskClass + '">' + esc(x.risk || '—') + '</span></td><td><span class="badge ' + baseClass + '">' + esc(x.baseline_status || '—') + '</span></td><td>' + esc(transport) + '</td><td title="' + esc(x.sni || '') + '">' + esc(x.sni || '—') + '</td><td title="' + esc(x.user_agent || '') + '">' + esc((x.user_agent || '—').slice(0,38)) + '</td><td>' + esc(point) + '</td></tr>'; }).join('') || '<tr><td colspan="13" class="empty">No client observations yet.</td></tr>');
    $('#browserSummary').html($.map(s.browsers, function (x) { var key = x.family.toLowerCase().replace(/[^a-z]+/g, '-'); var v = x.family.toLowerCase(), symbol = v.indexOf('android') === 0 ? 'android' : (v.indexOf('ios') === 0 ? 'phone_iphone' : 'public'); var share = s.total ? Math.round(x.count / s.total * 100) : 0; return '<div class="browser-item"><span class="material-symbols browser-icon ' + key.split('-').pop() + '">' + symbol + '</span><div><b>' + esc(x.family) + ' <strong>' + share + '%</strong></b><small>' + x.count + ' requests</small></div></div>'; }).join(''));
  }
  function refresh() { $.getJSON('/api/snapshot', render).done(function () { $('#mode').text('Live'); }).fail(function () { $('#mode').text('Reconnecting'); }); }
  refresh();
  window.setInterval(refresh, 5000);
  if (window.EventSource) {
    var source = new EventSource('/events');
    source.addEventListener('observation', refresh);
    source.addEventListener('ready', function () { $('#mode').text('Live'); });
    source.onerror = function () { $('#mode').text('Reconnecting'); };
  }
}(jQuery));`

func evidencePercent(s Snapshot) string {
	if s.Total == 0 {
		return "—"
	}
	n := s.Evidence.TLS + s.Evidence.ALPN + s.Evidence.ClientHello + s.Evidence.JA3 + s.Evidence.JA4 + s.Evidence.HTTP2
	return formatPercent(float64(n) / float64(s.Total*6) * 100)
}
func formatPercent(v float64) string { return fmt.Sprintf("%.0f%%", v) }
func statusClassName(status string) string {
	switch status {
	case "Normal":
		return "normal"
	case "Review":
		return "review"
	case "Degraded":
		return "degraded"
	default:
		return "insufficient"
	}
}
func barHeight(count, total int) int {
	if total == 0 {
		return 8
	}
	h := count * 100 / total
	if h < 8 {
		return 8
	}
	return h
}

func browserPercent(count, total int) string {
	if total <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%d%%", count*100/total)
}

func fingerprintCoverage(s Snapshot) string {
	if s.Total == 0 {
		return "—"
	}
	observed := s.Evidence.JA3 + s.Evidence.JA4 + s.Evidence.HTTP2
	return fmt.Sprintf("%d%%", observed*100/(s.Total*3))
}

func httpStatusClass(code int) string {
	switch {
	case code >= 500:
		return "s5"
	case code >= 400:
		return "s4"
	case code >= 300:
		return "s3"
	case code >= 200:
		return "s2"
	default:
		return "sx"
	}
}

func riskClass(value string) string {
	switch strings.ToLower(value) {
	case "high":
		return "high"
	case "review":
		return "review"
	case "low":
		return "low"
	default:
		return "info"
	}
}

func baselineClass(value string) string {
	switch strings.ToLower(value) {
	case "mismatch":
		return "mismatch"
	case "matched":
		return "matched"
	default:
		return "unknown"
	}
}

func shortURL(value string) string { return fingerprint.Truncate(value, 42) }

func transportLabel(tlsVersion, alpn string) string {
	if tlsVersion == "" && alpn == "" {
		return "—"
	}
	if tlsVersion == "" {
		return alpn
	}
	if strings.HasPrefix(tlsVersion, "TLS ") {
		tlsVersion = strings.TrimPrefix(tlsVersion, "TLS ")
	}
	if alpn == "" {
		return tlsVersion
	}
	return tlsVersion + " / " + alpn
}

func shortPoint(value string) string {
	switch value {
	case "nginx-http-module":
		return "nginx"
	case "go-tls":
		return "go-tls"
	case "go-http":
		return "go-http"
	default:
		return value
	}
}

func shortUA(value string) string {
	if value == "" {
		return "—"
	}
	return fingerprint.Truncate(value, 38)
}
