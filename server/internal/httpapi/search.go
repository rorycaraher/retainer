package httpapi

import (
	"database/sql"
	"net/http"

	"retainer/server/internal/models"
	"retainer/server/internal/notesvc"
	"retainer/server/internal/searchsvc"
)

func handleSearchNotes(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids, err := searchsvc.Search(db, r.URL.Query().Get("q"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		results := []*models.Note{}
		for _, id := range ids {
			n, err := notesvc.Get(db, id)
			if err != nil {
				continue // note vanished between the FTS match and this lookup — skip it
			}
			results = append(results, n)
		}

		writeJSON(w, http.StatusOK, map[string]any{"notes": results})
	}
}
