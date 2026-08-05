package webhandlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNotFoundRendersCustomPage(t *testing.T) {
	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, nil, nil)
	h := New(nil, tmpl, nil)
	req := httptest.NewRequest(http.MethodGet, "/ruta-inexistente", nil)
	rr := httptest.NewRecorder()

	h.NotFound(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	body := rr.Body.String()
	for _, want := range []string{"Página no encontrada", "/ruta-inexistente", "Ir al dashboard"} {
		if !strings.Contains(body, want) {
			t.Fatalf("custom 404 is missing %q:\n%s", want, body)
		}
	}
}
