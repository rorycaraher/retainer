package httpapi

import (
	"database/sql"
	"io"
	"net/http"
	"strconv"

	"retainer/server/internal/audiosvc"
	"retainer/server/internal/wsserver"
)

// maxUploadBytes is a generous ceiling on the raw multipart body — well
// above what 5 minutes of compressed voice audio should ever need. The real
// limit is the duration check below; this just bounds memory use against a
// wildly oversized or malformed request.
const maxUploadBytes = 20 * 1024 * 1024

// handleCreateAudioNote is the dedicated, one-shot creation path for Audio
// Notes (docs/adr/0005) — deliberately outside the generic /api/sync
// mutation batch, since the recording is immutable from the moment it's
// saved and there's no field to arbitrate via HLC.
func handleCreateAudioNote(db *sql.DB, hub *wsserver.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			writeError(w, http.StatusBadRequest, "invalid or oversized form data")
			return
		}

		id := r.FormValue("id")
		position := r.FormValue("position")
		if id == "" || position == "" {
			writeError(w, http.StatusBadRequest, "missing id or position")
			return
		}

		durationMs, err := strconv.ParseInt(r.FormValue("durationMs"), 10, 64)
		if err != nil || durationMs <= 0 {
			writeError(w, http.StatusBadRequest, "invalid durationMs")
			return
		}
		if durationMs > audiosvc.MaxDurationMs {
			writeError(w, http.StatusBadRequest, "recording exceeds the 5 minute limit")
			return
		}

		file, header, err := r.FormFile("audio")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing audio file")
			return
		}
		defer file.Close()

		mimeType := header.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "audio/webm"
		}

		blob, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read audio data")
			return
		}

		note, err := audiosvc.Create(db, id, r.FormValue("title"), position, mimeType, durationMs, blob)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var seq int64
		if err := db.QueryRow(`SELECT value FROM sync_counter WHERE id = 1`).Scan(&seq); err == nil {
			hub.Notify(seq)
		}

		writeJSON(w, http.StatusCreated, note)
	}
}

// handleGetAudio streams an Audio Note's recording. The blob is immutable
// once saved, so the response is safe to cache indefinitely.
func handleGetAudio(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		blob, mimeType, err := audiosvc.Audio(db, id)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		w.Write(blob)
	}
}
