package webhandlers

import (
	"net/http"

	"probakgo/internal/session"
)

func (h *WebH) IPBansPage(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	var bans any
	if h.ban != nil {
		bans = h.ban.ListBanned()
	}
	h.tmpl.Render(w, r, "ip_bans.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Bans":     bans,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) UnbanIPPost(w http.ResponseWriter, r *http.Request) {
	ip := r.FormValue("ip")
	if ip == "" || h.ban == nil {
		http.Redirect(w, r, "/settings/ip-bans", http.StatusSeeOther)
		return
	}
	h.ban.UnbanIP(ip)
	http.Redirect(w, r, "/settings/ip-bans?flash=IP+desbaneada&ok=1", http.StatusSeeOther)
}
