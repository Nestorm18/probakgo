package webhandlers

import (
	"net/http"
	"net/url"
	"strings"
)

func formBackOrDefault(r *http.Request, fallback string) string {
	back := strings.TrimSpace(r.FormValue("back"))
	if back == "" || !strings.HasPrefix(back, "/") || strings.HasPrefix(back, "//") {
		return fallback
	}
	if strings.ContainsAny(back, "\r\n") {
		return fallback
	}
	return back
}

func redirectWithFlash(w http.ResponseWriter, r *http.Request, target, message string, ok bool) {
	if target == "" {
		target = "/"
	}
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	if message != "" {
		target += sep + "flash=" + url.QueryEscape(message)
		sep = "&"
	}
	if ok {
		target += sep + "ok=1"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
