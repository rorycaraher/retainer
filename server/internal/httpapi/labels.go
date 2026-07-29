package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"retainer/server/internal/labelsvc"
)

func handleListLabels(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		labels, err := labelsvc.List(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"labels": labels})
	}
}

func handleCreateLabel(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		label, err := labelsvc.Create(db, in.Name, time.Now().UnixMilli())
		if errors.Is(err, labelsvc.ErrNameTaken) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, label)
	}
}

func handleRenameLabel(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var in struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		label, err := labelsvc.Rename(db, id, in.Name)
		if errors.Is(err, labelsvc.ErrNotFound) {
			writeError(w, http.StatusNotFound, "label not found")
			return
		}
		if errors.Is(err, labelsvc.ErrNameTaken) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, label)
	}
}

func handleDeleteLabel(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := labelsvc.Delete(db, id)
		if errors.Is(err, labelsvc.ErrNotFound) {
			writeError(w, http.StatusNotFound, "label not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
