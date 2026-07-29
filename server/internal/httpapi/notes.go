package httpapi

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"retainer/server/internal/notesvc"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	if status >= http.StatusInternalServerError {
		slog.Error("request failed", "status", status, "error", msg)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

func handleListNotes(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		notes, err := notesvc.List(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"notes": notes})
	}
}
