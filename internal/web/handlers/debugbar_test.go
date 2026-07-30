package webhandlers

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"probakgo/internal/session"
	"probakgo/internal/web/csp"
)

func TestDebugBarMiddlewareInjectsOnHTMLServerError(t *testing.T) {
	handler := DebugBarMiddleware(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("<html><body>broken</body></html>"))
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, authenticatedRequest(t, "/broken"))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rr.Body.String(), `id="pbk-dbg"`) {
		t.Fatal("debug bar was not injected into HTML error response")
	}
}

func TestTemplateErrorFallbackKeepsDebugBar(t *testing.T) {
	handler := DebugBarMiddleware(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderTemplateError(w, r, "alerts.html", "exec", errors.New(`wrong type for value; expected bool; got int`))
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, authenticatedRequest(t, "/alerts"))

	body := rr.Body.String()
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(body, "Error renderizando plantilla") {
		t.Fatal("template error fallback was not rendered")
	}
	if !strings.Contains(body, `id="pbk-dbg"`) {
		t.Fatal("debug bar was not injected into template error fallback")
	}
	if !strings.Contains(body, "template_error") {
		t.Fatal("template error was not recorded in debug vars")
	}
}

func TestDebugBarMiddlewareSkipsUnauthenticatedHTML(t *testing.T) {
	handler := DebugBarMiddleware(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>login</body></html>"))
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/login", nil))

	if strings.Contains(rr.Body.String(), `id="pbk-dbg"`) {
		t.Fatal("debug bar was injected for unauthenticated request")
	}
}

func TestDebugBarMiddlewareUsesRequestCSPNonce(t *testing.T) {
	handler := DebugBarMiddleware(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))

	req, nonce, err := csp.WithNonce(authenticatedRequest(t, "/"))
	if err != nil {
		t.Fatalf("WithNonce: %v", err)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	nonceAttr := `nonce="` + nonce + `"`
	if got := strings.Count(rr.Body.String(), nonceAttr); got != 2 {
		t.Fatalf("CSP nonce occurrences: got %d, want 2", got)
	}
}

func TestDebugBarMiddlewareBypassesDownloads(t *testing.T) {
	rr := httptest.NewRecorder()
	writtenDuringHandler := false
	handler := DebugBarMiddleware(true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("binary"))
		writtenDuringHandler = rr.Body.Len() == len("binary")
	}))

	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/download/client/linux-amd64", nil))

	if !writtenDuringHandler {
		t.Fatal("download response was buffered instead of streamed")
	}
	if got := rr.Body.String(); got != "binary" {
		t.Fatalf("body: got %q", got)
	}
}

func TestDebugBarMiddlewareStreamsOversizedHTMLWithoutInjection(t *testing.T) {
	handler := DebugBarMiddleware(true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxDebugResponseBytes+1))
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, authenticatedRequest(t, "/large"))

	if rr.Body.Len() != maxDebugResponseBytes+1 {
		t.Fatalf("body size: got %d", rr.Body.Len())
	}
	if strings.Contains(rr.Body.String(), `id="pbk-dbg"`) {
		t.Fatal("oversized response unexpectedly contains the debug bar")
	}
}

func TestDebugBarDurationWarningStartsAt200ms(t *testing.T) {
	okHTML := debugBarHTML(debugBarParams{
		elapsed: 199 * time.Millisecond,
		status:  http.StatusOK,
		method:  http.MethodGet,
		path:    "/",
		ct:      "text/html",
	})
	if !strings.Contains(okHTML, `<div><span class="pk">duration </span><span class="pv" style="color:#16a34a">199ms</span></div>`) {
		t.Fatal("expected 199ms to stay green")
	}

	warnHTML := debugBarHTML(debugBarParams{
		elapsed: 200 * time.Millisecond,
		status:  http.StatusOK,
		method:  http.MethodGet,
		path:    "/",
		ct:      "text/html",
	})
	if !strings.Contains(warnHTML, `<div><span class="pk">duration </span><span class="pv" style="color:#d97706">200ms</span></div>`) {
		t.Fatal("expected 200ms to be warning color")
	}
}

func TestDebugBarIsCompactResponsiveAndShowsSQLCount(t *testing.T) {
	html := debugBarHTML(debugBarParams{
		elapsed: 250 * time.Millisecond,
		status:  http.StatusOK,
		method:  http.MethodGet,
		path:    "/servers/pve",
		ct:      "text/html",
		queries: make([]string, 101),
	})

	for _, want := range []string{
		`aria-controls="pbk-dbg-body"`,
		`localStorage.getItem('pbk-dbg')==='1'`,
		`@media(max-width:900px)`,
		`justify-content:safe flex-end`,
		`.pbk-dbg-items{display:flex;align-items:center;min-width:max-content;margin-left:auto}`,
		`<span class="pbk-dbg-items">`,
		`style="color:#dc2626"><b>SQL</b> 101`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("debug bar missing %q", want)
		}
	}
	if strings.Contains(html, "%!") {
		t.Fatalf("debug bar contains a formatting error:\n%s", html)
	}
}

func authenticatedRequest(t *testing.T, path string) *http.Request {
	t.Helper()
	session.Init("01234567890123456789012345678901", false)

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	if err := session.SetUser(rr, req, "probakgo", "admin"); err != nil {
		t.Fatalf("set session: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, path, nil)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}
