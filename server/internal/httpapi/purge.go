package httpapi

import (
	"database/sql"
	"net/http"

	"retainer/server/internal/purge"
	"retainer/server/internal/wsserver"
)

// handlePurgeNote is the explicit "delete forever" action from the Trash
// view: immediate and irreversible, deliberately outside the generic sync
// mutation batch (a hard delete has no field to arbitrate via HLC).
func handlePurgeNote(db *sql.DB, hub *wsserver.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := purge.PurgeOne(db, id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var seq int64
		if err := db.QueryRow(`SELECT value FROM sync_counter WHERE id = 1`).Scan(&seq); err == nil {
			hub.Notify(seq)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
