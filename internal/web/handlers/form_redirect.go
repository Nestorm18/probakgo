package webhandlers

import (
	"mime"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

// SafeLocalPath returns raw when it is a same-origin absolute path, or "".
// Browsers read "/\host" as "//host" and drop tabs and newlines, so
// backslashes and control characters are rejected as well as "//".
func SafeLocalPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.ContainsRune(raw, '\\') {
		return ""
	}
	for _, c := range raw {
		if unicode.IsControl(c) {
			return ""
		}
	}
	if u, err := url.Parse(raw); err != nil || u.IsAbs() || u.Host != "" || u.User != nil {
		return ""
	}
	return raw
}

func formBackOrDefault(r *http.Request, fallback string) string {
	if back := SafeLocalPath(r.FormValue("back")); back != "" {
		return back
	}
	return fallback
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

// attachmentDisposition builds a Content-Disposition header for a download.
// File names can include client-reported host names, so quotes, separators
// and non-ASCII characters are encoded instead of concatenated.
func attachmentDisposition(filename string) string {
	if value := mime.FormatMediaType("attachment", map[string]string{"filename": filename}); value != "" {
		return value
	}
	return "attachment"
}
