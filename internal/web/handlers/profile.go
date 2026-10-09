package webhandlers

import (
	"encoding/base64"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/domain"
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
	telegramConfig, _ := h.store.GetTelegramConfig(ctx)
	telegramDestination, _ := h.store.GetTelegramDestinationForUser(ctx, user.ID)
	telegramPairingURL := ""
	telegramQRDataURI := template.URL("")
	if telegramDestination == nil {
		telegramPairingURL = h.telegramPairingURL(w, r, user.ID, telegramConfig)
		if telegramPairingURL != "" {
			telegramQRDataURI = qrCodeDataURI(telegramPairingURL)
		}
	}
	h.tmpl.Render(w, r, "profile.html", map[string]any{
		"Username":            username,
		"Role":                role,
		"User":                user,
		"TelegramConfig":      telegramConfig,
		"TelegramDestination": telegramDestination,
		"TelegramPairingURL":  telegramPairingURL,
		"TelegramQRDataURI":   telegramQRDataURI,
		"Flash":               r.URL.Query().Get("flash"),
		"FlashOK":             r.URL.Query().Get("ok") == "1",
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
		http.Redirect(w, r, "/profile?flash=La+nueva+contrase%C3%B1a+no+puede+estar+vac%C3%ADa", http.StatusSeeOther)
		return
	}
	if ok, blocked := h.checkCurrentPassword(r, user, currentPass); !ok {
		redirectWithFlash(w, r, "/profile", currentPasswordFailureMessage(blocked), false)
		return
	}
	if newPass != confirm {
		http.Redirect(w, r, "/profile?flash=Las+nuevas+contrase%C3%B1as+no+coinciden", http.StatusSeeOther)
		return
	}
	if len(newPass) < minPasswordLength {
		http.Redirect(w, r, "/profile?flash=La+contrase%C3%B1a+debe+tener+al+menos+12+caracteres", http.StatusSeeOther)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		redirectWithFlash(w, r, "/profile", passwordTooLongMessage, false)
		return
	}
	if err := h.store.UpdateUserPassword(ctx, user.ID, string(hash)); err != nil {
		slog.Error("update own password", "user_id", user.ID, "err", err)
		redirectWithFlash(w, r, "/profile", "No se pudo actualizar la contraseña", false)
		return
	}
	h.audit(r, "user.password_change", "user", strconv.FormatInt(user.ID, 10), user.Username, map[string]any{"self": true})
	h.redirectAfterOwnSecurityChange(w, r, user.ID, "Contraseña actualizada. Se han cerrado tus otras sesiones.")
}

// checkCurrentPassword verifies the signed-in user's password for a sensitive
// change. Failures are limited per user, so a stolen session cannot be used to
// brute-force the password.
func (h *WebH) checkCurrentPassword(r *http.Request, user *domain.User, password string) (ok, blocked bool) {
	key := strconv.FormatInt(user.ID, 10)
	if h.passwordFailures != nil && h.passwordFailures.Blocked(key) {
		return false, true
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) == nil {
		return true, false
	}
	if h.passwordFailures != nil {
		h.passwordFailures.AllowKey(key)
	}
	h.audit(r, "user.password_check_failed", "user", key, user.Username, map[string]any{"path": r.URL.Path})
	return false, false
}

func currentPasswordFailureMessage(blocked bool) string {
	if blocked {
		return "Demasiados intentos fallidos con la contraseña actual. Espera 15 minutos."
	}
	return "Contraseña actual incorrecta"
}

// redirectAfterOwnSecurityChange keeps the current browser signed in after a
// change that revoked every session of the user, including this one.
func (h *WebH) redirectAfterOwnSecurityChange(w http.ResponseWriter, r *http.Request, userID int64, message string) {
	user, err := h.store.GetUser(r.Context(), userID)
	if err == nil {
		err = session.SetUserWithVersion(w, r, user.ID, user.Username, user.Role, user.SessionVersion)
	}
	if err != nil {
		slog.Error("refresh session after security change", "user_id", userID, "err", err)
		session.Clear(w, r)
		redirectWithFlash(w, r, "/login", message+" Inicia sesión de nuevo.", false)
		return
	}
	redirectWithFlash(w, r, "/profile", message, true)
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
		slog.Error("generate TOTP secret", "err", err)
		redirectWithFlash(w, r, "/profile", "No se pudo preparar el 2FA", false)
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
		"QRDataURI": qrCodeDataURI(totp.ProvisioningURI(username, secret)),
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
	if user.TOTPEnabled {
		http.Redirect(w, r, "/profile?flash=2FA+ya+esta+activo", http.StatusSeeOther)
		return
	}
	secret, ok := session.GetPendingTOTPSetup(r)
	if !ok {
		http.Redirect(w, r, "/profile?flash=No+hay+configuraci%C3%B3n+2FA+pendiente", http.StatusSeeOther)
		return
	}
	step, valid := totp.ValidateStep(r.FormValue("code"), secret, time.Now())
	if !valid {
		h.tmpl.Render(w, r, "profile_2fa_setup.html", map[string]any{
			"Username":  username,
			"Role":      user.Role,
			"Secret":    secret,
			"URI":       totp.ProvisioningURI(username, secret),
			"QRDataURI": qrCodeDataURI(totp.ProvisioningURI(username, secret)),
			"Error":     "Código 2FA incorrecto",
		})
		return
	}
	if err := h.store.EnableUserTOTP(ctx, user.ID, secret); err != nil {
		slog.Error("enable TOTP", "user_id", user.ID, "err", err)
		redirectWithFlash(w, r, "/profile", "No se pudo activar el 2FA", false)
		return
	}
	// The confirmation code must not also work for the next login.
	if _, err := h.store.ClaimUserTOTPStep(ctx, user.ID, step); err != nil {
		slog.Warn("record TOTP setup step", "user_id", user.ID, "err", err)
	}
	h.audit(r, "user.2fa_enable", "user", username, username, nil)
	// The refreshed session also drops the pending TOTP setup.
	h.redirectAfterOwnSecurityChange(w, r, user.ID, "2FA activado. Se han cerrado tus otras sesiones.")
}

func (h *WebH) Profile2FADisable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, _, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}
	if ok, blocked := h.checkCurrentPassword(r, user, r.FormValue("current_password")); !ok {
		redirectWithFlash(w, r, "/profile", currentPasswordFailureMessage(blocked), false)
		return
	}
	if err := h.store.DisableUserTOTP(ctx, user.ID); err != nil {
		slog.Error("disable TOTP", "user_id", user.ID, "err", err)
		redirectWithFlash(w, r, "/profile", "No se pudo desactivar el 2FA", false)
		return
	}
	h.audit(r, "user.2fa_disable", "user", username, username, nil)
	h.redirectAfterOwnSecurityChange(w, r, user.ID, "2FA desactivado. Se han cerrado tus otras sesiones.")
}

func qrCodeDataURI(uri string) template.URL {
	png, err := qrcode.Encode(uri, qrcode.Medium, 220)
	if err != nil {
		return ""
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))
}
