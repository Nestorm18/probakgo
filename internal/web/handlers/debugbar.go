package webhandlers

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/debug"
	"probakgo/internal/session"
)

// DebugBarMiddleware injects an HTML debug bar at the bottom of every HTML response.
// Returns a no-op middleware when dev is false.
func DebugBarMiddleware(dev bool) func(http.Handler) http.Handler {
	if !dev {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := debug.NewContext(r.Context())

			bw := &bufferedWriter{ResponseWriter: w, status: http.StatusOK}
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
			})
			bw.flush([]byte(bar))
		})
	}
}

// bufferedWriter captures the response body so the middleware can inject HTML before flushing.
type bufferedWriter struct {
	http.ResponseWriter
	buf    bytes.Buffer
	status int
}

func (bw *bufferedWriter) WriteHeader(code int)        { bw.status = code }
func (bw *bufferedWriter) Write(b []byte) (int, error) { return bw.buf.Write(b) }
func (bw *bufferedWriter) Unwrap() http.ResponseWriter { return bw.ResponseWriter }

func (bw *bufferedWriter) flush(injection []byte) {
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
	case elapsedMs < 100:
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
		tmplDataIndicator = `<span class="pd">{ }</span>`
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

	return fmt.Sprintf(`<style>
#pbk-dbg{position:fixed;bottom:0;left:0;right:0;background:#f8fafc;color:#475569;font:11px/1 'Courier New',monospace;z-index:2147483647;border-top:2px solid #cbd5e1;box-shadow:0 -2px 8px rgba(0,0,0,.1)}
#pbk-dbg-bar{display:flex;align-items:center;cursor:pointer;user-select:none}
.pd{padding:5px 10px;border-right:1px solid #e2e8f0;white-space:nowrap}
#pbk-dbg-det{padding:8px 16px;background:#f1f5f9;line-height:2;display:grid;grid-template-columns:repeat(3,1fr);gap:0 24px}
.pk{color:#94a3b8}.pv{color:#1e293b;font-weight:500}
</style>
<div id="pbk-dbg">
<div id="pbk-dbg-bar">
<span class="pd" style="color:#3b82f6;font-weight:bold">◈ dev</span>
<span class="pd" style="color:%s;font-weight:bold">%d</span>
<span class="pd" style="color:%s">⏱ %s</span>
<span class="pd">💾 %s heap</span>
<span class="pd">📦 %s</span>
<span class="pd"><b>%s</b> %s</span>
<span class="pd">📄 %s</span>
<span class="pd">🧵 %d go</span>
%s
<span class="pd" style="margin-left:auto;border-left:1px solid #e2e8f0;border-right:none">👤 %s</span>
<span class="pd" id="pbk-dbg-arrow" style="border-right:none">▲</span>
</div>
<div id="pbk-dbg-body">
<details><summary style="%s"><span style="color:#475569;font-weight:600">request &amp; runtime</span> <span>%s %s · %s · %s heap</span></summary>
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
<script>
(function(){
  var dbg=document.getElementById('pbk-dbg');
  var body=document.getElementById('pbk-dbg-body');
  var arrow=document.getElementById('pbk-dbg-arrow');
  function syncPad(){
    var h=dbg.offsetHeight+'px';
    document.querySelectorAll('.sidebar,.main-content').forEach(function(el){el.style.paddingBottom=h});
  }
  var open=localStorage.getItem('pbk-dbg')!=='0';
  body.style.display=open?'block':'none';
  arrow.textContent=open?'▲':'▼';
  document.getElementById('pbk-dbg-bar').onclick=function(){
    var showing=body.style.display==='block';
    body.style.display=showing?'none':'block';
    arrow.textContent=showing?'▼':'▲';
    localStorage.setItem('pbk-dbg',showing?'0':'1');
    syncPad();
  };
  dbg.addEventListener('toggle',syncPad,true);
  syncPad();
})();
</script>`,
		// bar
		statusColor, p.status,
		dColor, dStr,
		heap,
		fmtBytesDebug(uint64(p.respSize)),
		p.method, p.path, tmplDisp,
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
		queriesDetail, varsDetail, tmplDataDetail)
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
