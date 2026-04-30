package web

// Session management is in internal/session. This file kept for package completeness.
// InitSessions delegates to session.Init.

import "probakgo/internal/session"

func InitSessions(key string, secure bool) {
	session.Init(key, secure)
}
