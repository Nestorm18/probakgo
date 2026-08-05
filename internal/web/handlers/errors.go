package webhandlers

import (
	"net/http"

	"probakgo/internal/session"
)

func (h *WebH) NotFound(w http.ResponseWriter, r *http.Request) {
	_, _, loggedIn := session.GetUser(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	h.tmpl.Render(w, r, "not_found.html", map[string]any{
		"Path":     r.URL.Path,
		"LoggedIn": loggedIn,
	})
}
