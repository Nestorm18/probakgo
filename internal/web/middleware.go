package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/session"
	"probakgo/internal/store"
	"probakgo/internal/totp"
)

// RequireLogin checks the session cookie and verifies the user is still active in DB.
// If the user's role changed since login, the session is refreshed so downstream
// middleware (RequireEditor, RequireAdmin) see the current role.
func RequireLogin(st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, sessionRole, ok := session.GetUser(r)
			if !ok {
				http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusSeeOther)
				return
			}
			user, err := st.GetUserByUsername(r.Context(), username)
			if err != nil || !user.IsActive {
				session.Clear(w, r)
				http.Redirect(w, r, "/login?flash=Tu+sesión+ha+sido+invalidada", http.StatusSeeOther)
				return
			}
			if sessionVersion, ok := session.UserVersion(r); !ok || sessionVersion != user.SessionVersion {
				session.Clear(w, r)
				http.Redirect(w, r, "/login?flash=Tu+sesion+ha+sido+invalidada", http.StatusSeeOther)
				return
			}
			if user.Role != sessionRole {
				_ = session.SetUserWithVersion(w, r, username, user.Role, user.SessionVersion)
			}
			if userNeedsTOTPEnforcement(st, r, user) {
				_ = st.SetUserActive(r.Context(), user.ID, false)
				session.Clear(w, r)
				http.Redirect(w, r, "/login?flash=Usuario+desactivado:+2FA+no+se+activo+dentro+del+plazo", http.StatusSeeOther)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			next.ServeHTTP(w, r)
		})
	}
}

func RequireTOTPForSensitiveAction(st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, _, ok := session.GetUser(r)
			if !ok {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			cfg, err := st.GetEmailConfig(r.Context())
			if err != nil || cfg == nil || !cfg.SensitiveActionsRequireTOTP {
				next.ServeHTTP(w, r)
				return
			}
			user, err := st.GetUserByUsername(r.Context(), username)
			if err != nil || !user.IsActive {
				session.Clear(w, r)
				http.Redirect(w, r, "/login?flash=Tu+sesion+ha+sido+invalidada", http.StatusSeeOther)
				return
			}
			if !user.TOTPEnabled {
				if wantsJSON(r) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "Esta operacion requiere 2FA activo"})
					return
				}
				http.Redirect(w, r, "/profile?flash=Esta+operacion+requiere+2FA+activo", http.StatusSeeOther)
				return
			}
			now := time.Now()
			if session.SensitiveTOTPFresh(r, now) {
				next.ServeHTTP(w, r)
				return
			}
			code := strings.TrimSpace(r.FormValue("totp_code"))
			if code != "" && totp.Validate(code, user.TOTPSecret, now) {
				_ = session.SetSensitiveTOTPFresh(w, r, now.Add(5*time.Minute))
				next.ServeHTTP(w, r)
				return
			}
			if wantsJSON(r) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Esta operacion requiere un codigo 2FA valido"})
				return
			}
			http.Redirect(w, r, r.URL.Path+"?flash=Codigo+2FA+requerido+para+esta+operacion", http.StatusSeeOther)
		})
	}
}

func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json") ||
		strings.HasSuffix(r.URL.Path, "/reveal")
}

func userNeedsTOTPEnforcement(st *store.Store, r *http.Request, user *domain.User) bool {
	if user.Role == "reader" || user.TOTPEnabled {
		return false
	}
	cfg, err := st.GetEmailConfig(r.Context())
	if err != nil || cfg == nil || !cfg.EnforceTOTPNonReaders {
		return false
	}
	if user.TOTPGraceStartedAt == nil {
		_ = st.StartUserTOTPGrace(r.Context(), user.ID)
		return false
	}
	return time.Since(*user.TOTPGraceStartedAt) >= 72*time.Hour
}

// RequireEditor allows admin and editor roles.
func RequireEditor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, role, ok := session.GetUser(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if role != "admin" && role != "editor" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin allows only admin role.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, role, ok := session.GetUser(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
