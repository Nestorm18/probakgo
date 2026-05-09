package api

import (
	"net/http"

	"probakgo/internal/service"
)

type contextKey string

const ctxAPIKey contextKey = "api_key"

func (s *Server) requireServerKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := service.ExtractBearer(r.Header.Get("Authorization"))
		machineID := r.Header.Get("X-Machine-ID")
		k, err := s.auth.ValidateServerKey(raw, machineID)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, err.Error())
			return
		}
		r = r.WithContext(withValue(r.Context(), ctxAPIKey, k))
		next.ServeHTTP(w, r)
	})
}

