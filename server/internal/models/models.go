// Package models holds the core domain types shared across services.
package models

// Note is either a Text Note (Body is used) or a Checklist (Items is used),
// per CONTEXT.md — never both at once, but convertible between the two.
type Note struct {
	ID              string          `json:"id"`
	Kind            string          `json:"kind"` // "text" | "checklist"
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
