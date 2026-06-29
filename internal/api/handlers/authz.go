package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"probakgo/internal/api/apictx"
)

func (h *H) requireKeyServer(w http.ResponseWriter, r *http.Request, serverName string) bool {
	k, ok := apictx.APIKey(r.Context())
	if !ok {
		errJSON(w, http.StatusUnauthorized, "invalid or inactive API key")
		return false
	}
	serverName = strings.TrimSpace(serverName)
	boundServerName := strings.TrimSpace(k.ServerName)
	if serverName == "" {
		errJSON(w, http.StatusBadRequest, "server name is required")
		return false
	}
	if boundServerName == "" {
		if err := h.store.BindAPIKeyServerName(r.Context(), k.ID, serverName); err != nil {
			internalErr(w, "bind api key server", err)
			return false
		}
		k.ServerName = serverName
		return true
	}
	if boundServerName != serverName {
		errJSON(w, http.StatusForbidden, "API key is bound to a different server: expected "+strconv.Quote(boundServerName)+", got "+strconv.Quote(serverName))
		return false
	}
	return true
}
