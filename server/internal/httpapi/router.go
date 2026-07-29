// Package httpapi wires HTTP routes to the underlying services.
package httpapi

import (
	"database/sql"
	"net/http"

	"retainer/server/internal/authsvc"
	"retainer/server/internal/wsserver"
)

// NewRouter builds the top-level HTTP handler. hub must already be running
// (its Run loop started by the caller) — NewRouter only wires handlers to it.
func NewRouter(db *sql.DB, staticDir string, hub *wsserver.Hub) http.Handler {
	mux := http.NewServeMux()
	limiter := authsvc.NewLoginLimiter()

	mux.HandleFunc("GET /health", handleHealth(db))

	mux.HandleFunc("POST /api/auth/login", handleLogin(db, limiter))
	mux.HandleFunc("POST /api/auth/logout", handleLogout(db))
	mux.Handle("GET /api/auth/status", requireAuth(db, handleAuthStatus()))

	mux.Handle("GET /api/notes", requireAuth(db, handleListNotes(db)))
	mux.Handle("GET /api/notes/search", requireAuth(db, handleSearchNotes(db)))
	mux.Handle("POST /api/sync", requireAuth(db, handleSync(db, hub)))
	mux.Handle("DELETE /api/notes/{id}/purge", requireAuth(db, handlePurgeNote(db, hub)))
	mux.Handle("GET /ws", requireAuth(db, handleWS(hub)))

	mux.Handle("GET /api/labels", requireAuth(db, handleListLabels(db)))
	mux.Handle("POST /api/labels", requireAuth(db, handleCreateLabel(db)))
	mux.Handle("PATCH /api/labels/{id}", requireAuth(db, handleRenameLabel(db)))
	mux.Handle("DELETE /api/labels/{id}", requireAuth(db, handleDeleteLabel(db)))

	if staticDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(staticDir)))
	}

	return mux
}
