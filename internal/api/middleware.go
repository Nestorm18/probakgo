package api

import (
	"net/http"
	"strconv"

	"probakgo/internal/api/apictx"
	"probakgo/internal/service"
)

func (s *Server) requireServerKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := service.ExtractBearer(r.Header.Get("Authorization"))
		machineID := r.Header.Get("X-Machine-ID")
		k, err := s.auth.ValidateServerKey(raw, machineID)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, err.Error())
			return
		}
		if s.clientLimiter != nil && !s.clientLimiter.AllowKey(strconv.FormatInt(k.ID, 10)) {
			w.Header().Set("Retry-After", "60")
			jsonError(w, http.StatusTooManyRequests, "too many requests for this API key")
			return
		}
		r = r.WithContext(apictx.WithAPIKey(r.Context(), k))
		next.ServeHTTP(w, r)
	})
}
