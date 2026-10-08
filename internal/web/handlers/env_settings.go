package webhandlers

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"probakgo/internal/config"
	"probakgo/internal/netutil"
)

func (h *WebH) EnableSessionSecurePost(w http.ResponseWriter, r *http.Request) {
	if h.tmpl != nil && h.tmpl.secure {
		http.Redirect(w, r, "/settings/system?flash=SESSION_SECURE+ya+esta+activo&ok=1", http.StatusSeeOther)
		return
	}
	if netutil.RequestScheme(r) != "https" {
		http.Redirect(w, r, "/settings/system?flash=Entra+por+HTTPS+para+activar+SESSION_SECURE+automaticamente", http.StatusSeeOther)
		return
	}
	path, err := setServerEnvValue("SESSION_SECURE", "true")
	if err != nil {
		http.Redirect(w, r, "/settings/system?flash=Error+actualizando+.env:+"+urlFlash(err.Error()), http.StatusSeeOther)
		return
	}
	h.audit(r, "settings.session_secure_enable", "settings", "system", "SESSION_SECURE", map[string]any{"env_path": path})
	http.Redirect(w, r, "/settings/system?flash=SESSION_SECURE=true+guardado.+Reinicia+probakgo+para+aplicarlo&ok=1", http.StatusSeeOther)
}

func setServerEnvValue(key, value string) (string, error) {
	path := serverEnvPath()
	if err := config.SetEnvFileValue(path, key, value); err != nil {
		return "", err
	}
	return path, nil
}

func serverEnvPath() string {
	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, ".env"))
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ".env"
}

func urlFlash(s string) string {
	return url.QueryEscape(s)
}
