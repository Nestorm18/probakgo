package web

// Session management is in internal/session. This file kept for package completeness.
// InitSessions delegates to session.Init.

import "probaky/internal/session"

func InitSessions(key string) {
	session.Init(key)
}
