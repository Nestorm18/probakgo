package webhandlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"probakgo/internal/domain"
	"probakgo/internal/ratelimit"
	"probakgo/internal/session"
)

const auditLogPageSize = 25

func (h *WebH) AuditLogPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	rows, err := h.store.ListAuditLogsPage(ctx, auditLogPageSize+1, (page-1)*auditLogPageSize)
	if err != nil {
		slog.Error("list audit log", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	hasNext := len(rows) > auditLogPageSize
	if hasNext {
		rows = rows[:auditLogPageSize]
	}

	users, err := h.store.ListUsers(ctx)
	if err != nil {
		slog.Error("list users for audit log", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "audit_log.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Rows":     rows,
		"Users":    users,

		"AuditPage":     page,
		"AuditPrevPage": page - 1,
		"AuditNextPage": page + 1,
		"AuditHasPrev":  page > 1,
		"AuditHasNext":  hasNext,
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
