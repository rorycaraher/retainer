export type NoteKind = 'text' | 'checklist'

export interface ChecklistItem {
  id: string
  noteId: string
  text: string
  checked: boolean
  deleted: boolean
  position: string
  serverSeq: number
}

export interface Note {
  id: string
  kind: NoteKind
  title: string
  body: string
  color: string
  pinned: boolean
  archived: boolean
  trashedAt?: number | null
  position: string
  archivePosition: string
  serverSeq: number
  createdAt: number
  updatedAt: number
  items: ChecklistItem[]
  labelIds: string[]
}

export interface Label {
  id: string
  name: string
  createdAt: number
}
