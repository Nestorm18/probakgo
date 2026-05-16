package webhandlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"probakgo/internal/domain"
	"probakgo/internal/ratelimit"
	"probakgo/internal/session"
)

func (h *WebH) AuditLogPage(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	rows, err := h.store.ListAuditLogs(r.Context(), 200)
	if err != nil {
		slog.Error("list audit log", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "audit_log.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Rows":     rows,
	})
}

func (h *WebH) audit(r *http.Request, action, targetType, targetID, targetName string, meta map[string]any) {
	username, role, _ := session.GetUser(r)
	metadata := "{}"
	if len(meta) > 0 {
		if b, err := json.Marshal(meta); err == nil {
			metadata = string(b)
		}
	}
	if err := h.store.InsertAuditLog(r.Context(), domain.AuditLog{
		ActorUsername: username,
		ActorRole:     role,
		ActorIP:       ratelimit.ExtractIP(r),
		Action:        action,
		TargetType:    targetType,
		TargetID:      targetID,
		TargetName:    targetName,
		Metadata:      metadata,
	}); err != nil {
		slog.Warn("insert audit log", "err", err, "action", action)
	}
}
