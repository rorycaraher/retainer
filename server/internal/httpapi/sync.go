package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"retainer/server/internal/syncsvc"
	"retainer/server/internal/wsserver"
)

func handleSync(db *sql.DB, hub *wsserver.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			SinceSeq  int64              `json:"sinceSeq"`
			Mutations []syncsvc.Mutation `json:"mutations"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		seq, changes, err := syncsvc.Apply(db, in.SinceSeq, in.Mutations)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if len(in.Mutations) > 0 {
			hub.Notify(seq)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"serverSeq": seq,
			"changes":   changes,
		})
	}
}
