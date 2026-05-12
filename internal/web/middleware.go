package web

import (
	"net/http"

	"probakgo/internal/session"
	"probakgo/internal/store"
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
			if user.Role != sessionRole {
				_ = session.SetUser(w, r, username, user.Role)
			}
			next.ServeHTTP(w, r)
		})
	}
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
