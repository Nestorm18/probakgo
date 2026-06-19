package webhandlers

import (
	"net/http"
	"sort"
	"strconv"

	"probakgo/internal/session"
)

const ipBansPageSize = 25

func (h *WebH) IPBansPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	bansPage, _ := strconv.Atoi(r.URL.Query().Get("bans_page"))
	if bansPage < 1 {
		bansPage = 1
	}
	attemptsPage, _ := strconv.Atoi(r.URL.Query().Get("attempts_page"))
	if attemptsPage < 1 {
		attemptsPage = 1
	}

	var bans any
	bansHasNext := false
	if h.ban != nil {
		allBans := h.ban.ListBanned()
		sort.SliceStable(allBans, func(i, j int) bool {
			if allBans[i].BannedAt.Equal(allBans[j].BannedAt) {
				return allBans[i].IP < allBans[j].IP
			}
			return allBans[i].BannedAt.After(allBans[j].BannedAt)
		})
		offset := (bansPage - 1) * ipBansPageSize
		if offset < len(allBans) {
			end := offset + ipBansPageSize + 1
			if end > len(allBans) {
				end = len(allBans)
			}
			pageRows := allBans[offset:end]
			bansHasNext = len(pageRows) > ipBansPageSize
			if bansHasNext {
				pageRows = pageRows[:ipBansPageSize]
			}
			bans = pageRows
		} else {
			bans = allBans[:0]
		}
	}
	attempts, _ := h.store.ListLoginAttemptsPage(ctx, ipBansPageSize+1, (attemptsPage-1)*ipBansPageSize)
	attemptsHasNext := len(attempts) > ipBansPageSize
	if attemptsHasNext {
		attempts = attempts[:ipBansPageSize]
	}
	h.tmpl.Render(w, r, "ip_bans.html", map[string]any{
		"Username":      username,
		"Role":          role,
		"Bans":          bans,
		"LoginAttempts": attempts,
		"Flash":         r.URL.Query().Get("flash"),
		"FlashOK":       r.URL.Query().Get("ok") == "1",

		"BansPage":         bansPage,
		"BansPrevPage":     bansPage - 1,
		"BansNextPage":     bansPage + 1,
		"BansHasPrev":      bansPage > 1,
		"BansHasNext":      bansHasNext,
		"AttemptsPage":     attemptsPage,
		"AttemptsPrevPage": attemptsPage - 1,
		"AttemptsNextPage": attemptsPage + 1,
		"AttemptsHasPrev":  attemptsPage > 1,
		"AttemptsHasNext":  attemptsHasNext,
	})
}

func (h *WebH) UnbanIPPost(w http.ResponseWriter, r *http.Request) {
	ip := r.FormValue("ip")
	if ip == "" || h.ban == nil {
		http.Redirect(w, r, "/settings/ip-bans", http.StatusSeeOther)
		return
	}
	h.ban.UnbanIP(ip)
	h.audit(r, "ip_ban.unban", "ip_ban", ip, ip, nil)
	http.Redirect(w, r, "/settings/ip-bans?flash=IP+desbaneada&ok=1", http.StatusSeeOther)
}
