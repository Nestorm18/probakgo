package webhandlers

import (
	"encoding/base64"
	"html/template"
	"net/http"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/session"
	"probakgo/internal/totp"
)

func (h *WebH) Profile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "profile.html", map[string]any{
		"Username": username,
		"Role":     role,
		"User":     user,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) ProfilePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, _, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	currentPass := r.FormValue("current_password")
	newPass := r.FormValue("new_password")
	confirm := r.FormValue("password_confirm")

	if newPass == "" {
		http.Redirect(w, r, "/profile?flash=La+nueva+contrasena+no+puede+estar+vacia", http.StatusSeeOther)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPass)) != nil {
		http.Redirect(w, r, "/profile?flash=Contrasena+actual+incorrecta", http.StatusSeeOther)
		return
	}
	if newPass != confirm {
		http.Redirect(w, r, "/profile?flash=Las+nuevas+contrasenas+no+coinciden", http.StatusSeeOther)
		return
	}
	if len(newPass) < minPasswordLength {
		http.Redirect(w, r, "/profile?flash=La+contrasena+debe+tener+al+menos+12+caracteres", http.StatusSeeOther)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		http.Redirect(w, r, "/profile?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	if err := h.store.UpdateUserPassword(ctx, user.ID, string(hash)); err != nil {
		http.Redirect(w, r, "/profile?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/profile?flash=Contrasena+actualizada&ok=1", http.StatusSeeOther)
}

func (h *WebH) Profile2FASetup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}
	if user.TOTPEnabled {
		http.Redirect(w, r, "/profile?flash=2FA+ya+esta+activo", http.StatusSeeOther)
		return
	}
	secret, err := totp.GenerateSecret()
	if err != nil {
		http.Redirect(w, r, "/profile?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	if err := session.SetPendingTOTPSetup(w, r, secret); err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "profile_2fa_setup.html", map[string]any{
		"Username":  username,
		"Role":      role,
		"Secret":    secret,
		"URI":       totp.ProvisioningURI(username, secret),
		"QRDataURI": totpQRCodeDataURI(totp.ProvisioningURI(username, secret)),
	})
}

func (h *WebH) Profile2FAConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, _, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}
	secret, ok := session.GetPendingTOTPSetup(r)
	if !ok {
		http.Redirect(w, r, "/profile?flash=No+hay+configuracion+2FA+pendiente", http.StatusSeeOther)
		return
	}
	if !totp.Validate(r.FormValue("code"), secret, time.Now()) {
		h.tmpl.Render(w, r, "profile_2fa_setup.html", map[string]any{
			"Username":  username,
			"Role":      user.Role,
			"Secret":    secret,
			"URI":       totp.ProvisioningURI(username, secret),
			"QRDataURI": totpQRCodeDataURI(totp.ProvisioningURI(username, secret)),
			"Error":     "Codigo 2FA incorrecto",
		})
		return
	}
	if err := h.store.EnableUserTOTP(ctx, user.ID, secret); err != nil {
		http.Redirect(w, r, "/profile?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	_ = session.ClearPendingTOTPSetup(w, r)
	h.audit(r, "user.2fa_enable", "user", username, username, nil)
	http.Redirect(w, r, "/profile?flash=2FA+activado&ok=1", http.StatusSeeOther)
}

func (h *WebH) Profile2FADisable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, _, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(r.FormValue("current_password"))) != nil {
		http.Redirect(w, r, "/profile?flash=Contrasena+actual+incorrecta", http.StatusSeeOther)
		return
	}
	if err := h.store.DisableUserTOTP(ctx, user.ID); err != nil {
		http.Redirect(w, r, "/profile?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	h.audit(r, "user.2fa_disable", "user", username, username, nil)
	http.Redirect(w, r, "/profile?flash=2FA+desactivado&ok=1", http.StatusSeeOther)
}

func totpQRCodeDataURI(uri string) template.URL {
	png, err := qrcode.Encode(uri, qrcode.Medium, 220)
	if err != nil {
		return ""
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))
}
