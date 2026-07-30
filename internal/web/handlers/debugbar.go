package webhandlers

import (
	"bytes"
	"fmt"
	"net/http"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/debug"
	"probakgo/internal/session"
	"probakgo/internal/web/csp"
)

const maxDebugResponseBytes = 1 << 20

// DebugBarMiddleware injects an HTML debug bar at the bottom of every HTML response.
// Returns a no-op middleware when dev is false.
func DebugBarMiddleware(dev bool) func(http.Handler) http.Handler {
	if !dev {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if debugBarBypassRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			ctx := debug.NewContext(r.Context())

			bw := &bufferedWriter{ResponseWriter: w, status: http.StatusOK, limit: maxDebugResponseBytes}
			next.ServeHTTP(bw, r.WithContext(ctx))

			elapsed := time.Since(start)

			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)

			username, role, loggedIn := session.GetUser(r)
			if !loggedIn {
				bw.flush(nil)
				return
			}

			di := debug.FromContext(ctx)
			di.Mu.Lock()
			tmplName := di.Template
			queries := di.Queries
			vars := di.Vars
			tmplData := di.TemplateData
			di.Mu.Unlock()

			routePattern := ""
			if rc := chi.RouteContext(ctx); rc != nil {
				routePattern = rc.RoutePattern()
			}

			ct := bw.Header().Get("Content-Type")
			bar := debugBarHTML(debugBarParams{
				elapsed:   elapsed,
				ms:        ms,
				method:    r.Method,
				path:      r.URL.Path,
				query:     r.URL.RawQuery,
				route:     routePattern,
				tmpl:      tmplName,
				username:  username,
				role:      role,
				status:    bw.status,
				respSize:  bw.buf.Len(),
				ct:        ct,
				queries:   queries,
				vars:      vars,
				tmplData:  tmplData,
				userAgent: r.UserAgent(),
				nonce:     csp.Nonce(r),
			})
			bw.flush([]byte(bar))
		})
	}
}

// bufferedWriter captures the response body so the middleware can inject HTML before flushing.
type bufferedWriter struct {
	http.ResponseWriter
	buf         bytes.Buffer
	status      int
	limit       int
	wroteHeader bool
	passthrough bool
}

func (bw *bufferedWriter) WriteHeader(code int) {
	if bw.wroteHeader {
		return
	}
	bw.wroteHeader = true
	bw.status = code
	if bw.passthrough {
		bw.ResponseWriter.WriteHeader(code)
	}
}

func (bw *bufferedWriter) Write(b []byte) (int, error) {
	if !bw.wroteHeader {
		bw.WriteHeader(http.StatusOK)
	}
	if bw.passthrough {
		return bw.ResponseWriter.Write(b)
	}
	if bw.limit > 0 && bw.buf.Len()+len(b) > bw.limit {
		bw.passthrough = true
		bw.ResponseWriter.WriteHeader(bw.status)
		if _, err := bw.ResponseWriter.Write(bw.buf.Bytes()); err != nil {
			return 0, err
		}
		bw.buf.Reset()
		return bw.ResponseWriter.Write(b)
	}
	return bw.buf.Write(b)
}

func (bw *bufferedWriter) Unwrap() http.ResponseWriter { return bw.ResponseWriter }

func (bw *bufferedWriter) flush(injection []byte) {
	if bw.passthrough {
		return
	}
	ct := bw.Header().Get("Content-Type")
	bw.ResponseWriter.WriteHeader(bw.status)
	body := bw.buf.Bytes()
	if strings.Contains(ct, "text/html") && len(injection) > 0 {
		if idx := bytes.LastIndex(body, []byte("</body>")); idx >= 0 {
			var nb []byte
			nb = append(nb, body[:idx]...)
			nb = append(nb, injection...)
			nb = append(nb, body[idx:]...)
			body = nb
		} else {
			body = append(body, injection...)
		}
	}
	_, _ = bw.ResponseWriter.Write(body)
}

func debugBarBypassRequest(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/static/") || strings.HasPrefix(r.URL.Path, "/download/") {
		return true
	}
	switch strings.ToLower(path.Ext(r.URL.Path)) {
	case ".csv", ".json":
		return true
	default:
		return false
	}
}

type debugBarParams struct {
	elapsed   time.Duration
	ms        runtime.MemStats
	method    string
	path      string
	query     string
	route     string
	tmpl      string
	username  string
	role      string
	status    int
	respSize  int
	ct        string
	queries   []string
	vars      []debug.DebugVar
	tmplData  string
	userAgent string
	nonce     string
}

func debugBarHTML(p debugBarParams) string {
	elapsedMs := p.elapsed.Milliseconds()
	var dStr string
	if elapsedMs < 1 {
		dStr = fmt.Sprintf("%dµs", p.elapsed.Microseconds())
	} else {
		dStr = fmt.Sprintf("%dms", elapsedMs)
	}
	var dColor string
	switch {
	case elapsedMs < 200:
		dColor = "#16a34a"
	case elapsedMs < 500:
		dColor = "#d97706"
	default:
		dColor = "#dc2626"
	}

	var statusColor string
	switch {
	case p.status < 300:
		statusColor = "#16a34a"
	case p.status < 400:
		statusColor = "#d97706"
	default:
		statusColor = "#dc2626"
	}

	queryColor := "#64748b"
	switch {
	case len(p.queries) > 100:
		queryColor = "#dc2626"
	case len(p.queries) > 30:
		queryColor = "#d97706"
	case len(p.queries) > 0:
		queryColor = "#16a34a"
	}

	heap := fmtBytesDebug(p.ms.HeapAlloc)
	sys := fmtBytesDebug(p.ms.Sys)
	stack := fmtBytesDebug(p.ms.StackInuse)
	totalAlloc := fmtBytesDebug(p.ms.TotalAlloc)

	tmplDisp := p.tmpl
	if tmplDisp == "" {
		tmplDisp = "–"
	}
	userDisp := "–"
	if p.username != "" {
		userDisp = p.username + " (" + p.role + ")"
	}
	routeDisp := p.route
	if routeDisp == "" {
		routeDisp = p.path
	}
	queryDisp := "–"
	if p.query != "" {
		queryDisp = "?" + p.query
	}
	ctDisp := p.ct
	if ctDisp == "" {
		ctDisp = "–"
	}
	uaDisp := p.userAgent
	if uaDisp == "" {
		uaDisp = "–"
	}

	goroutines := runtime.NumGoroutine()
	goVersion := runtime.Version()
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()

	// last GC pause in µs
	var lastGCPause string
	if p.ms.NumGC > 0 {
		idx := (p.ms.NumGC + 255) % 256
		pauseNs := p.ms.PauseNs[idx]
		lastGCPause = fmt.Sprintf("%dµs", pauseNs/1000)
	} else {
		lastGCPause = "–"
	}

	// SQL queries panel
	var queriesHTML string
	if len(p.queries) == 0 {
		queriesHTML = `<div><span class="pk">queries </span><span class="pv">–</span></div>`
	} else {
		queriesHTML = fmt.Sprintf(`<div><span class="pk">queries </span><span class="pv">%d</span></div>`, len(p.queries))
	}

	subPanelStyle := `border-top:1px solid #e2e8f0;font-size:10px`
	summaryStyle := `padding:4px 16px;cursor:pointer;color:#94a3b8;user-select:none;list-style:none;display:flex;align-items:center;gap:6px`
	summaryStyle += `;outline:none`

	var queriesDetail string
	if len(p.queries) > 0 {
		var sb strings.Builder
		fmt.Fprintf(&sb, `<details style="%s"><summary style="%s">`, subPanelStyle, summaryStyle)
		fmt.Fprintf(&sb, `<span style="color:#475569;font-weight:600">SQL</span> <span>%d queries</span></summary>`, len(p.queries))
		sb.WriteString(`<div style="padding:4px 16px 8px;max-height:140px;overflow-y:auto">`)
		for i, q := range p.queries {
			fmt.Fprintf(&sb, `<div style="margin:2px 0"><span style="color:#94a3b8">%d.</span> <span style="color:#1e293b">%s</span></div>`, i+1, htmlEscape(q))
		}
		sb.WriteString(`</div></details>`)
		queriesDetail = sb.String()
	}

	var varsDetail string
	if len(p.vars) > 0 {
		var sb strings.Builder
		fmt.Fprintf(&sb, `<details style="%s"><summary style="%s">`, subPanelStyle, summaryStyle)
		fmt.Fprintf(&sb, `<span style="color:#475569;font-weight:600">vars</span> <span>%d</span></summary>`, len(p.vars))
		sb.WriteString(`<div style="padding:4px 16px 8px;max-height:140px;overflow-y:auto;display:grid;grid-template-columns:auto 1fr;gap:2px 12px">`)
		for _, v := range p.vars {
			fmt.Fprintf(&sb, `<span style="color:#94a3b8;white-space:nowrap">%s</span><span style="color:#1e293b;word-break:break-all">%s</span>`, htmlEscape(v.Key), htmlEscape(v.Value))
		}
		sb.WriteString(`</div></details>`)
		varsDetail = sb.String()
	}

	var tmplDataDetail string
	tmplDataIndicator := ""
	if p.tmplData != "" {
		tmplDataIndicator = `<span class="pd pbk-dbg-hide-md">{ }</span>`
		var sb strings.Builder
		fmt.Fprintf(&sb, `<details style="%s;background:#fafaf0"><summary style="%s">`, subPanelStyle, summaryStyle)
		sb.WriteString(`<span style="color:#475569;font-weight:600">template data</span></summary>`)
		sb.WriteString(`<div style="padding:4px 16px 8px">`)
		sb.WriteString(`<pre style="margin:0;max-height:200px;overflow-y:auto;color:#1e293b;white-space:pre-wrap;word-break:break-all">`)
		sb.WriteString(htmlEscape(p.tmplData))
		sb.WriteString(`</pre></div></details>`)
		tmplDataDetail = sb.String()
	}

	detSummaryStyle := `padding:4px 16px;cursor:pointer;color:#94a3b8;user-select:none;list-style:none;display:flex;align-items:center;gap:6px;border-top:1px solid #e2e8f0;outline:none`

	nonceAttr := ""
	if p.nonce != "" {
		nonceAttr = ` nonce="` + htmlEscape(p.nonce) + `"`
	}

	return fmt.Sprintf(`<style%s>
#pbk-dbg,#pbk-dbg *{box-sizing:border-box}
#pbk-dbg{position:fixed;bottom:0;left:0;right:0;background:#f8fafc;color:#475569;font:11px/1.25 'Courier New',monospace;z-index:2147483647;border-top:2px solid #cbd5e1;box-shadow:0 -2px 8px rgba(15,23,42,.12)}
#pbk-dbg-bar{display:flex;align-items:center;justify-content:safe flex-end;width:100%%;min-height:29px;padding:0;border:0;background:transparent;color:inherit;font:inherit;text-align:left;cursor:pointer;user-select:none;overflow-x:auto;scrollbar-width:thin}
#pbk-dbg .pbk-dbg-items{display:flex;align-items:center;min-width:max-content;margin-left:auto}
#pbk-dbg-bar:focus-visible{outline:2px solid #3b82f6;outline-offset:-2px}
#pbk-dbg .pd{flex:0 0 auto;padding:7px 10px;border-right:1px solid #e2e8f0;white-space:nowrap}
#pbk-dbg .pbk-dbg-grow{flex:1 1 auto;min-width:0;overflow:hidden;text-overflow:ellipsis}
#pbk-dbg-body{max-height:min(62vh,560px);overflow:auto;border-top:1px solid #e2e8f0}
#pbk-dbg details>summary::before{content:"›";font-family:inherit;margin-right:6px;font-size:14px;line-height:1;transition:transform .15s;display:inline-block}
#pbk-dbg details[open]>summary::before{transform:rotate(90deg)}
#pbk-dbg-det{padding:9px 16px;background:#f1f5f9;line-height:2;display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:0 24px}
#pbk-dbg .pk{color:#94a3b8}#pbk-dbg .pv{color:#1e293b;font-weight:500}
@media(max-width:900px){#pbk-dbg .pbk-dbg-hide-md{display:none}#pbk-dbg-det{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:620px){#pbk-dbg .pbk-dbg-hide-sm{display:none}#pbk-dbg-det{grid-template-columns:1fr}#pbk-dbg .pd{padding-inline:8px}}
</style>
<div id="pbk-dbg">
<button type="button" id="pbk-dbg-bar" aria-controls="pbk-dbg-body" aria-expanded="false">
<span class="pbk-dbg-items">
<span class="pd" style="color:#3b82f6;font-weight:bold">◈ dev</span>
<span class="pd" style="color:%s;font-weight:bold">%d</span>
<span class="pd" style="color:%s">⏱ %s</span>
<span class="pd pbk-dbg-hide-md">💾 %s heap</span>
<span class="pd pbk-dbg-hide-sm">📦 %s</span>
<span class="pd pbk-dbg-grow"><b>%s</b> %s</span>
<span class="pd pbk-dbg-hide-sm">📄 %s</span>
<span class="pd" style="color:%s"><b>SQL</b> %d</span>
<span class="pd pbk-dbg-hide-md">🧵 %d go</span>
%s
<span class="pd pbk-dbg-hide-sm" style="margin-left:auto;border-left:1px solid #e2e8f0;border-right:none">👤 %s</span>
<span class="pd" id="pbk-dbg-arrow" style="border-right:none">▲</span>
</span>
</button>
<div id="pbk-dbg-body">
<details open><summary style="%s"><span style="color:#475569;font-weight:600">request &amp; runtime</span> <span>%s %s · %s · %s heap</span></summary>
<div id="pbk-dbg-det">
<div><span class="pk">method </span><span class="pv">%s</span></div>
<div><span class="pk">status </span><span class="pv" style="color:%s">%d</span></div>
<div><span class="pk">route </span><span class="pv">%s</span></div>
<div><span class="pk">path </span><span class="pv">%s</span></div>
<div><span class="pk">query </span><span class="pv">%s</span></div>
<div><span class="pk">content-type </span><span class="pv">%s</span></div>
<div><span class="pk">template </span><span class="pv">%s</span></div>
<div><span class="pk">resp size </span><span class="pv">%s</span></div>
<div><span class="pk">user </span><span class="pv">%s</span></div>
<div><span class="pk">duration </span><span class="pv" style="color:%s">%s</span></div>
<div><span class="pk">heap alloc </span><span class="pv">%s</span></div>
<div><span class="pk">total alloc </span><span class="pv">%s</span></div>
<div><span class="pk">sys mem </span><span class="pv">%s</span></div>
<div><span class="pk">stack </span><span class="pv">%s</span></div>
<div><span class="pk">goroutines </span><span class="pv">%d</span></div>
<div><span class="pk">gc runs </span><span class="pv">%d</span></div>
<div><span class="pk">last gc pause </span><span class="pv">%s</span></div>
%s
<div><span class="pk">go </span><span class="pv">%s</span></div>
<div><span class="pk">GOMAXPROCS </span><span class="pv">%d / %d CPUs</span></div>
<div><span class="pk">user-agent </span><span class="pv" style="font-size:10px;word-break:break-all">%s</span></div>
</div>
</details>
%s
%s
%s
</div>
</div>
<script%s>
(function(){
  var dbg=document.getElementById('pbk-dbg');
  var body=document.getElementById('pbk-dbg-body');
  var bar=document.getElementById('pbk-dbg-bar');
  var arrow=document.getElementById('pbk-dbg-arrow');
  function syncPad(){
    var h=dbg.offsetHeight+'px';
    document.querySelectorAll('.sidebar,.main-content').forEach(function(el){el.style.paddingBottom=h});
  }
  var open=localStorage.getItem('pbk-dbg')==='1';
  body.style.display=open?'block':'none';
  arrow.textContent=open?'▲':'▼';
  bar.setAttribute('aria-expanded',open?'true':'false');
  bar.onclick=function(){
    var showing=body.style.display==='block';
    body.style.display=showing?'none':'block';
    arrow.textContent=showing?'▼':'▲';
    bar.setAttribute('aria-expanded',showing?'false':'true');
    localStorage.setItem('pbk-dbg',showing?'0':'1');
    syncPad();
  };
  dbg.addEventListener('toggle',syncPad,true);
  if(window.ResizeObserver){new ResizeObserver(syncPad).observe(dbg)}
  window.addEventListener('resize',syncPad);
  syncPad();
})();
</script>`,
		nonceAttr,
		// bar
		statusColor, p.status,
		dColor, dStr,
		heap,
		fmtBytesDebug(uint64(p.respSize)),
		p.method, p.path, tmplDisp,
		queryColor, len(p.queries),
		goroutines,
		tmplDataIndicator,
		userDisp,
		// request & runtime details summary
		detSummaryStyle, p.method, routeDisp, dStr, heap,
		// detail grid
		p.method, statusColor, p.status, routeDisp,
		p.path, queryDisp, ctDisp,
		tmplDisp, fmtBytesDebug(uint64(p.respSize)), userDisp,
		dColor, dStr,
		heap, totalAlloc,
		sys, stack,
		goroutines,
		p.ms.NumGC, lastGCPause,
		queriesHTML,
		goVersion, maxProcs, numCPU,
		uaDisp,
		queriesDetail, varsDetail, tmplDataDetail,
		nonceAttr)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&#34;")
	return s
}

func fmtBytesDebug(b uint64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
