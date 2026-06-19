package webhandlers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"probakgo/internal/selfupdate"
	"probakgo/internal/session"
)

func (h *WebH) About(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)

	pveServers, _ := h.store.ListPVEServers(ctx)
	pbsServers, _ := h.store.ListPBSServers(ctx)
	windowsServers, _ := h.store.ListWindowsServers(ctx)
	dbSize := h.store.DBSize(ctx)

	h.tmpl.Render(w, r, "about.html", map[string]any{
		"Username":     username,
		"Role":         role,
		"PVECount":     len(pveServers),
		"PBSCount":     len(pbsServers),
		"WindowsCount": len(windowsServers),
		"DBSize":       dbSize,
		"Uptime":       uptimeStr(time.Since(h.startTime)),
		"StartTime":    h.startTime,
	})
}

func (h *WebH) AboutUpdatePost(w http.ResponseWriter, r *http.Request) {
	latest, err := selfupdate.LatestTag("Nestorm18/probakgo")
	if err != nil {
		redirectFlash(w, r, "No se pudo comprobar la version online: "+err.Error(), false)
		return
	}
	newer, ok := selfupdate.IsNewer(latest, h.tmpl.version)
	if !ok {
		redirectFlash(w, r, "No se pudo comparar la version local con la online", false)
		return
	}
	if !newer {
		redirectFlash(w, r, "Probakgo ya esta totalmente actualizado ("+h.tmpl.version+")", true)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		redirectFlash(w, r, "No se pudo localizar el binario", false)
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	cmd := exec.Command(exe, "update")
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		redirectFlash(w, r, "No se pudo iniciar la actualizacion", false)
		return
	}
	redirectFlash(w, r, "Hay una nueva version ("+latest+"). Actualizacion iniciada; el servicio se reiniciara al instalarla.", true)
}

func redirectFlash(w http.ResponseWriter, r *http.Request, msg string, ok bool) {
	q := url.Values{}
	q.Set("flash", msg)
	if ok {
		q.Set("ok", "1")
	}
	http.Redirect(w, r, "/about?"+q.Encode(), http.StatusSeeOther)
}

func uptimeStr(d time.Duration) string {
	d = d.Truncate(time.Second)
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	if hours >= 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, mins, secs)
	}
	return fmt.Sprintf("%dm %ds", mins, secs)
}
