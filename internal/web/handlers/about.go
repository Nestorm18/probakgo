package webhandlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"probakgo/internal/session"
)

func (h *WebH) About(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)

	pveServers, _ := h.store.ListPVEServers(ctx)
	pbsServers, _ := h.store.ListPBSServers(ctx)
	dbSize := h.store.DBSize(ctx)

	h.tmpl.Render(w, r, "about.html", map[string]any{
		"Username":  username,
		"Role":      role,
		"PVECount":  len(pveServers),
		"PBSCount":  len(pbsServers),
		"DBSize":    dbSize,
		"Uptime":    uptimeStr(time.Since(h.startTime)),
		"StartTime": h.startTime,
	})
}

func (h *WebH) AboutUpdatePost(w http.ResponseWriter, r *http.Request) {
	exe, err := os.Executable()
	if err != nil {
		http.Redirect(w, r, "/about?flash=No+se+pudo+localizar+el+binario", http.StatusSeeOther)
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	cmd := exec.Command(exe, "update")
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		http.Redirect(w, r, "/about?flash=No+se+pudo+iniciar+la+actualizacion", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/about?flash=Buscando+actualizaciones.+Si+hay+una+nueva+version,+el+servicio+se+reiniciara.&ok=1", http.StatusSeeOther)
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
