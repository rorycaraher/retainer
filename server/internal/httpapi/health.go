package httpapi

import (
	"database/sql"
	"net/http"
)

// handleHealth is deliberately unauthenticated — used by uptime checks and
// (eventually) container orchestration, both of which run before any login.
func handleHealth(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
