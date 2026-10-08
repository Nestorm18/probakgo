package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"probakgo/internal/api/apictx"
	"probakgo/internal/domain"
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
		bound, err := h.store.BindAPIKeyServerName(r.Context(), k.ID, serverName)
		if err != nil {
			internalErr(w, "bind api key server", err)
			return false
		}
		if bound {
			k.ServerName = serverName
			return true
		}
		// A concurrent request bound the key first; validate against it.
		current, err := h.store.GetAPIKey(r.Context(), k.ID)
		if err != nil {
			internalErr(w, "reload api key server", err)
			return false
		}
		k.ServerName = current.ServerName
		boundServerName = strings.TrimSpace(current.ServerName)
	}
	if boundServerName != serverName {
		errJSON(w, http.StatusForbidden, "API key is bound to a different server: expected "+strconv.Quote(boundServerName)+", got "+strconv.Quote(serverName))
		return false
	}
	return true
}

// keyMayReadServer is the read-only form of requireKeyServer: a key bound to
// another server name is rejected, and a read never binds an unbound key.
func keyMayReadServer(w http.ResponseWriter, k *domain.APIKey, serverName string) bool {
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		errJSON(w, http.StatusBadRequest, "server name is required")
		return false
	}
	bound := strings.TrimSpace(k.ServerName)
	if bound != "" && bound != serverName {
		errJSON(w, http.StatusForbidden, "API key is bound to a different server: expected "+strconv.Quote(bound)+", got "+strconv.Quote(serverName))
		return false
	}
	return true
}

// validVMID accepts Proxmox VM IDs, which range from 100 to 999999999.
func validVMID(vmid string) bool {
	n, err := strconv.Atoi(vmid)
	return err == nil && n >= 100 && n <= 999999999 && strconv.Itoa(n) == vmid
}
