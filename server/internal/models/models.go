// Package models holds the core domain types shared across services.
package models

// Note is a Text Note (Body used), a Checklist (Items used), or an Audio
// Note (AudioMimeType/AudioDurationMs used), per CONTEXT.md — exactly one at
// once. The recording itself isn't on this struct: it's fetched separately
// via GET /api/notes/{id}/audio so list/sync payloads stay small.
type Note struct {
	ID              string          `json:"id"`
	Kind            string          `json:"kind"` // "text" | "checklist" | "audio"
	Title           string          `json:"title"`
	Body            string          `json:"body"`
	Color           string          `json:"color"`
	Pinned          bool            `json:"pinned"`
	Archived        bool            `json:"archived"`
	TrashedAt       *int64          `json:"trashedAt,omitempty"`
	Position        string          `json:"position"`
	ArchivePosition string          `json:"archivePosition"`
	ServerSeq       int64           `json:"serverSeq"`
	CreatedAt       int64           `json:"createdAt"`
	UpdatedAt       int64           `json:"updatedAt"`
	Items           []ChecklistItem `json:"items"`
	LabelIDs        []string        `json:"labelIds"`
	AudioMimeType   string          `json:"audioMimeType,omitempty"`
	AudioDurationMs int64           `json:"audioDurationMs,omitempty"`
}

// ChecklistItem is a single line within a Checklist.
type ChecklistItem struct {
	ID        string `json:"id"`
	NoteID    string `json:"noteId"`
	Text      string `json:"text"`
	Checked   bool   `json:"checked"`
	Deleted   bool   `json:"deleted"`
	Position  string `json:"position"`
	ServerSeq int64  `json:"serverSeq"`
}

// Label is a first-class, user-managed tag.
type Label struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
}
