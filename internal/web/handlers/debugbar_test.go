package webhandlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"probakgo/internal/session"
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
