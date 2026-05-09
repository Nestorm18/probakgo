package webhandlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/session"
)

type debugKey struct{}

type debugInfo struct {
	mu       sync.Mutex
	Template string
}

func debugFromContext(ctx context.Context) *debugInfo {
	v, _ := ctx.Value(debugKey{}).(*debugInfo)
	return v
}

// DebugBarMiddleware injects an HTML debug bar at the bottom of every HTML response.
// Returns a no-op middleware when dev is false.
func DebugBarMiddleware(dev bool) func(http.Handler) http.Handler {
	if !dev {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			di := &debugInfo{}
			ctx := context.WithValue(r.Context(), debugKey{}, di)

			bw := &bufferedWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(bw, r.WithContext(ctx))

			elapsed := time.Since(start)

			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)

			username, role, _ := session.GetUser(r)

			di.mu.Lock()
			tmplName := di.Template
			di.mu.Unlock()

			routePattern := ""
			if rc := chi.RouteContext(ctx); rc != nil {
				routePattern = rc.RoutePattern()
			}

			bar := debugBarHTML(elapsed, ms.HeapAlloc, r.Method, r.URL.Path, routePattern, tmplName, username, role)
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

func (bw *bufferedWriter) WriteHeader(code int) { bw.status = code }
func (bw *bufferedWriter) Write(b []byte) (int, error) { return bw.buf.Write(b) }
func (bw *bufferedWriter) Unwrap() http.ResponseWriter  { return bw.ResponseWriter }

func (bw *bufferedWriter) flush(injection []byte) {
	ct := bw.Header().Get("Content-Type")
	bw.ResponseWriter.WriteHeader(bw.status)
	body := bw.buf.Bytes()
	if bw.status == http.StatusOK && strings.Contains(ct, "text/html") && len(injection) > 0 {
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

func debugBarHTML(elapsed time.Duration, heapAlloc uint64, method, path, route, tmpl, user, role string) string {
	ms := elapsed.Milliseconds()
	var dStr string
	if ms < 1 {
		dStr = fmt.Sprintf("%dµs", elapsed.Microseconds())
	} else {
		dStr = fmt.Sprintf("%dms", ms)
	}
	var dColor string
	switch {
	case ms < 100:
		dColor = "#4ade80"
	case ms < 500:
		dColor = "#fb923c"
	default:
		dColor = "#f87171"
	}

	heap := fmtBytesDebug(heapAlloc)
	tmplDisp := tmpl
	if tmplDisp == "" {
		tmplDisp = "–"
	}
	userDisp := "–"
	if user != "" {
		userDisp = user + " (" + role + ")"
	}
	routeDisp := route
	if routeDisp == "" {
		routeDisp = path
	}
	goroutines := runtime.NumGoroutine()

	return fmt.Sprintf(`<style>
#pbk-dbg{position:fixed;bottom:0;left:0;right:0;background:#0f172a;color:#94a3b8;font:11px/1 'Courier New',monospace;z-index:2147483647;border-top:1px solid #1e293b}
#pbk-dbg-bar{display:flex;align-items:center;cursor:pointer}
.pd{padding:5px 10px;border-right:1px solid #1e293b;white-space:nowrap}
#pbk-dbg-det{display:none;border-top:1px solid #1e293b;padding:8px 16px;background:#070d1a;line-height:2;columns:2}
.pk{color:#475569}.pv{color:#e2e8f0}
</style>
<div id="pbk-dbg">
<div id="pbk-dbg-bar" onclick="var d=document.getElementById('pbk-dbg-det');d.style.display=d.style.display==='block'?'none':'block'">
<span class="pd" style="color:#60a5fa;font-weight:bold">◈ dev</span>
<span class="pd" style="color:%s">⏱ %s</span>
<span class="pd">💾 %s</span>
<span class="pd">%s %s</span>
<span class="pd">📄 %s</span>
<span class="pd" style="margin-left:auto;border-left:1px solid #1e293b;border-right:none">👤 %s</span>
<span class="pd" style="border-right:none">▼</span>
</div>
<div id="pbk-dbg-det">
<div><span class="pk">method </span><span class="pv">%s</span></div>
<div><span class="pk">path </span><span class="pv">%s</span></div>
<div><span class="pk">route </span><span class="pv">%s</span></div>
<div><span class="pk">template </span><span class="pv">%s</span></div>
<div><span class="pk">duration </span><span class="pv">%s</span></div>
<div><span class="pk">heap </span><span class="pv">%s</span></div>
<div><span class="pk">goroutines </span><span class="pv">%d</span></div>
<div><span class="pk">user </span><span class="pv">%s</span></div>
</div>
</div>`,
		dColor, dStr, heap,
		method, path, tmplDisp, userDisp,
		method, path, routeDisp, tmplDisp, dStr, heap, goroutines, userDisp)
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
