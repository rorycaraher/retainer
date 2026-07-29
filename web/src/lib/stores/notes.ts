import { derived, get, writable } from 'svelte/store'
import type { ChecklistItem, Note } from '../types'
import * as api from '../api/rest'
import * as hlc from '../hlc'
import { getDeviceId } from '../deviceId'
import { keyBetween } from '../fracindex'

// allNotes holds EVERYTHING — pinned, unpinned, archived, trashed — the same
// way an /api/sync response's `changes` does. Main/Archive/Trash views are
// plain client-side filters over this one source of truth, so reconciling a
// sync response never needs to know which view is currently on screen.
export const allNotes = writable<Note[]>([])
export const loadError = writable<string | null>(null)

// Set to a note's id right after it's created so its card can auto-focus its
// title field once, then cleared — see NoteCard's autofocusIfNew action.
export const justCreatedId = writable<string | null>(null)

// Same idea, one level down: the id of a checklist item just created via
// addItem/addItemAfter, so its text field can auto-focus.
export const justCreatedItemId = writable<string | null>(null)

function isInMain(n: Note): boolean {
  return !n.archived && !n.trashedAt
}

// Notes sort newest/top-of-list first by DESCENDING position; items within a
// checklist sort ASCENDING (oldest/first-added at the top).
function sortByPositionDesc(list: Note[]): Note[] {
  return [...list].sort((a, b) => (a.position < b.position ? 1 : a.position > b.position ? -1 : 0))
}

function sortItems(items: ChecklistItem[]): ChecklistItem[] {
  return [...items].sort((a, b) => (a.position < b.position ? -1 : a.position > b.position ? 1 : 0))
}

const mainNotes = derived(allNotes, (all) => sortByPositionDesc(all.filter(isInMain)))
export const pinnedNotes = derived(mainNotes, (list) => list.filter((n) => n.pinned))
export const unpinnedNotes = derived(mainNotes, (list) => list.filter((n) => !n.pinned))
export const archivedNotes = derived(allNotes, (all) =>
  [...all.filter((n) => n.archived && !n.trashedAt)].sort((a, b) =>
    a.archivePosition < b.archivePosition ? 1 : a.archivePosition > b.archivePosition ? -1 : 0,
  ),
)
export const trashedNotes = derived(allNotes, (all) =>
  [...all.filter((n) => n.trashedAt)].sort((a, b) => (b.trashedAt ?? 0) - (a.trashedAt ?? 0)),
)

let lastSyncedSeq = 0
let pending: api.Mutation[] = []
let flushTimer: ReturnType<typeof setTimeout> | null = null

export async function load() {
  try {
    const { notes: fetched } = await api.listNotes()
    let maxSeq = 0
    for (const n of fetched) {
      maxSeq = Math.max(maxSeq, n.serverSeq)
      for (const it of n.items) maxSeq = Math.max(maxSeq, it.serverSeq)
    }
    lastSyncedSeq = maxSeq
    allNotes.set(fetched)
    loadError.set(null)
  } catch (e) {
    loadError.set(e instanceof Error ? e.message : String(e))
  }
}

function queue(m: api.Mutation) {
  pending.push(m)
  if (flushTimer) clearTimeout(flushTimer)
  flushTimer = setTimeout(syncNow, 400)
}

// queueBundle stamps every mutation with the SAME HLC value, so they're
// arbitrated as one atomic edit against concurrent field writes from other
// devices — e.g. pin+unarchive together, or an edit that should implicitly
// revive a note/item a concurrent trash/delete raced against (see the
// plan's "delete-vs-edit" convention).
function queueBundle(muts: Array<Omit<api.Mutation, 'hlc'>>) {
  const stamp = hlc.next(getDeviceId())
  for (const m of muts) queue({ ...m, hlc: stamp })
}

// syncNow pushes any queued local edits (if any) and pulls anything new from
// the server in the same call. It's the one code path behind both the
// debounced local-edit flush and the WebSocket's "go pull now" signal.
export async function syncNow() {
  if (flushTimer) {
    clearTimeout(flushTimer)
    flushTimer = null
  }
  const batch = pending
  pending = []
  try {
    const res = await api.sync(lastSyncedSeq, batch)
    reconcile(res)
  } catch (e) {
    // Failed mutations go back on the queue so they aren't silently lost;
    // the next successful syncNow (debounced edit, WS pull, or reconnect
    // catch-up) will retry them.
    pending = [...batch, ...pending]
    loadError.set(e instanceof Error ? e.message : String(e))
  }
}

function reconcile(res: api.SyncResponse) {
  lastSyncedSeq = Math.max(lastSyncedSeq, res.serverSeq)
  allNotes.update((list) => {
    const byId = new Map(list.map((n) => [n.id, n]))
    for (const n of res.changes.notes) {
      const existing = byId.get(n.id)
      byId.set(n.id, { ...n, items: existing?.items ?? [], labelIds: existing?.labelIds ?? [] })
    }
    for (const it of res.changes.items) {
      const note = byId.get(it.noteId)
      if (!note) continue
      const items = note.items.filter((existing) => existing.id !== it.id)
      if (!it.deleted) items.push(it)
      byId.set(note.id, { ...note, items: sortItems(items) })
    }
    for (const nl of res.changes.noteLabels) {
      const note = byId.get(nl.noteId)
      if (!note) continue
      const labelIds = note.labelIds.filter((id) => id !== nl.labelId)
      if (nl.attached) labelIds.push(nl.labelId)
      byId.set(note.id, { ...note, labelIds })
    }
    return Array.from(byId.values())
  })
}

function updateLocalNote(id: string, patch: Partial<Note>) {
  allNotes.update((all) => all.map((n) => (n.id === id ? { ...n, ...patch } : n)))
}

function mutateNote(id: string, field: string, value: unknown) {
  queue({ entity: 'note', id, field, value, hlc: hlc.next(getDeviceId()) })
}

function mutateItem(id: string, noteId: string, field: string, value: unknown) {
  queue({ entity: 'item', id, noteId, field, value, hlc: hlc.next(getDeviceId()) })
}

export async function addTextNote(title: string, body: string) {
  const id = crypto.randomUUID()
  const now = Date.now()
  const topPosition = get(mainNotes)[0]?.position ?? ''
  const position = keyBetween(topPosition, '') // new notes go to the top
  const optimistic: Note = {
    id,
    kind: 'text',
    title,
    body,
    color: 'default',
    pinned: false,
    archived: false,
    position,
    archivePosition: '',
    serverSeq: 0,
    createdAt: now,
    updatedAt: now,
    items: [],
    labelIds: [],
  }
  allNotes.update((list) => [optimistic, ...list])
  justCreatedId.set(id)
  mutateNote(id, 'kind', 'text')
  mutateNote(id, 'title', title)
  mutateNote(id, 'body', body)
  mutateNote(id, 'position', position)
}

export async function addChecklist(title: string) {
  const id = crypto.randomUUID()
  const now = Date.now()
  const topPosition = get(mainNotes)[0]?.position ?? ''
  const position = keyBetween(topPosition, '')
  const optimistic: Note = {
    id,
    kind: 'checklist',
    title,
    body: '',
    color: 'default',
    pinned: false,
    archived: false,
    position,
    archivePosition: '',
    serverSeq: 0,
    createdAt: now,
    updatedAt: now,
    items: [],
    labelIds: [],
  }
  allNotes.update((list) => [optimistic, ...list])
  justCreatedId.set(id)
  mutateNote(id, 'kind', 'checklist')
  mutateNote(id, 'title', title)
  mutateNote(id, 'position', position)
}

// setNoteField covers plain content edits. It always bundles trashedAt:null
// at the same HLC as the edit — the "edit implicitly revives if newer"
// convention: if this edit genuinely postdates a concurrent trash from
// another device, it un-trashes the note; if it's older, the trash wins as
// normal HLC arbitration. Pin/Archive go through dedicated toggle functions
// below since they carry cross-field invariants of their own.
export function setNoteField(note: Note, field: 'title' | 'body' | 'color', value: string) {
  updateLocalNote(note.id, { [field]: value } as Partial<Note>)
  queueBundle([
    { entity: 'note', id: note.id, field, value },
    { entity: 'note', id: note.id, field: 'trashedAt', value: null },
  ])
}

// togglePinned enforces Pinned/Archived mutual exclusivity (CONTEXT.md):
// pinning a note un-archives it. Both fields are stamped with the same HLC
// so concurrent pin-on-device-A / archive-on-device-B races resolve to one
// consistent state rather than a split "both true" outcome.
export function togglePinned(note: Note) {
  if (note.pinned) {
    updateLocalNote(note.id, { pinned: false })
    mutateNote(note.id, 'pinned', false)
    return
  }
  updateLocalNote(note.id, { pinned: true, archived: false })
  queueBundle([
    { entity: 'note', id: note.id, field: 'pinned', value: true },
    { entity: 'note', id: note.id, field: 'archived', value: false },
  ])
}

export function toggleArchived(note: Note) {
  if (note.archived) {
    updateLocalNote(note.id, { archived: false })
    mutateNote(note.id, 'archived', false)
    return
  }
  const topArchivePosition = get(archivedNotes)[0]?.archivePosition ?? ''
  const archivePosition = keyBetween(topArchivePosition, '')
  updateLocalNote(note.id, { archived: true, pinned: false, archivePosition })
  queueBundle([
    { entity: 'note', id: note.id, field: 'archived', value: true },
    { entity: 'note', id: note.id, field: 'pinned', value: false },
    { entity: 'note', id: note.id, field: 'archivePosition', value: archivePosition },
  ])
}

export function trashNote(note: Note) {
  const trashedAt = Date.now()
  updateLocalNote(note.id, { trashedAt })
  mutateNote(note.id, 'trashedAt', trashedAt)
}

export function restoreNote(note: Note) {
  updateLocalNote(note.id, { trashedAt: null })
  mutateNote(note.id, 'trashedAt', null)
}

// purgeNoteForever is the explicit "delete forever" action: immediate,
// irreversible, and deliberately NOT part of the generic mutation batch (a
// hard delete has no field to arbitrate via HLC against concurrent edits).
export async function purgeNoteForever(note: Note) {
  await api.purgeNote(note.id)
  allNotes.update((all) => all.filter((n) => n.id !== note.id))
}

// reorderNote/reorderArchivedNote take the CALLER's already-filtered,
// already-sorted view list (e.g. $pinnedNotes, $unpinnedNotes, or
// $archivedNotes) — dragging only ever reorders within one such group, never
// across them (crossing groups goes through the pin/archive toggles above).
function reorder(list: Note[], draggedId: string, targetIndex: number, field: 'position' | 'archivePosition') {
  const dragged = list.find((n) => n.id === draggedId)
  if (!dragged) return
  const rest = list.filter((n) => n.id !== draggedId)
  const index = Math.max(0, Math.min(targetIndex, rest.length))
  const above = rest[index - 1] // higher value (lists are DESC-sorted)
  const below = rest[index]
  const value = keyBetween(below?.[field] ?? '', above?.[field] ?? '')
  updateLocalNote(draggedId, { [field]: value } as Partial<Note>)
  mutateNote(draggedId, field, value)
}

export function reorderNote(list: Note[], draggedId: string, targetIndex: number) {
  reorder(list, draggedId, targetIndex, 'position')
}

export function reorderArchivedNote(list: Note[], draggedId: string, targetIndex: number) {
  reorder(list, draggedId, targetIndex, 'archivePosition')
}

function insertItem(note: Note, position: string) {
  const id = crypto.randomUUID()
  const optimistic: ChecklistItem = {
    id,
    noteId: note.id,
    text: '',
    checked: false,
    deleted: false,
    position,
    serverSeq: 0,
  }
  allNotes.update((list) =>
    list.map((n) => (n.id === note.id ? { ...n, items: sortItems([...n.items, optimistic]) } : n)),
  )
  justCreatedItemId.set(id)
  mutateItem(id, note.id, 'text', '')
  mutateItem(id, note.id, 'position', position)
}

export function addItem(note: Note) {
  const lastPosition = note.items[note.items.length - 1]?.position ?? ''
  insertItem(note, keyBetween(lastPosition, '')) // new items go to the bottom
}

// addItemAfter inserts a new empty item directly below afterItem — used when
// pressing Enter in an item's text field, so it behaves like a normal
// checklist/text editor ("Enter" continues the list at that point, not at
// the bottom).
export function addItemAfter(note: Note, afterItem: ChecklistItem) {
  const items = sortItems(note.items)
  const index = items.findIndex((it) => it.id === afterItem.id)
  const next = index >= 0 ? items[index + 1] : undefined
  insertItem(note, keyBetween(afterItem.position, next?.position ?? ''))
}

// setItemField bundles deleted:false at the same HLC as the edit — same
// "edit implicitly revives if newer" convention as setNoteField, one level
// down: a concurrent edit genuinely newer than another device's delete
// revives the item (with the edit intact).
export function setItemField(note: Note, item: ChecklistItem, field: 'text' | 'checked', value: string | boolean) {
  allNotes.update((list) =>
    list.map((n) =>
      n.id !== note.id ? n : { ...n, items: n.items.map((it) => (it.id === item.id ? { ...it, [field]: value } : it)) },
    ),
  )
  queueBundle([
    { entity: 'item', id: item.id, noteId: note.id, field, value },
    { entity: 'item', id: item.id, noteId: note.id, field: 'deleted', value: false },
  ])
}

export function removeItem(note: Note, item: ChecklistItem) {
  allNotes.update((list) =>
    list.map((n) => (n.id !== note.id ? n : { ...n, items: n.items.filter((it) => it.id !== item.id) })),
  )
  mutateItem(item.id, note.id, 'deleted', true)
}

export function attachLabel(note: Note, labelId: string) {
  if (note.labelIds.includes(labelId)) return
  updateLocalNote(note.id, { labelIds: [...note.labelIds, labelId] })
  queue({ entity: 'note_label', noteId: note.id, labelId, field: 'attached', value: true, hlc: hlc.next(getDeviceId()) })
}

export function detachLabel(note: Note, labelId: string) {
  updateLocalNote(note.id, { labelIds: note.labelIds.filter((id) => id !== labelId) })
  queue({ entity: 'note_label', noteId: note.id, labelId, field: 'attached', value: false, hlc: hlc.next(getDeviceId()) })
}

// stripLabelFromAllNotes keeps this client's own local state consistent
// right after deleting a label (a plain REST hard delete — see labelsvc —
// so it isn't visible to OTHER connected clients until they reload).
export function stripLabelFromAllNotes(labelId: string) {
  allNotes.update((all) => all.map((n) => (n.labelIds.includes(labelId) ? { ...n, labelIds: n.labelIds.filter((id) => id !== labelId) } : n)))
}

export function reorderItem(note: Note, draggedId: string, targetIndex: number) {
  const dragged = note.items.find((it) => it.id === draggedId)
  if (!dragged) return
  const rest = note.items.filter((it) => it.id !== draggedId)
  const index = Math.max(0, Math.min(targetIndex, rest.length))
  const before = rest[index - 1] // lower position (items are ASC-sorted)
  const after = rest[index]
  const position = keyBetween(before?.position ?? '', after?.position ?? '')
  const updated = { ...dragged, position }
  rest.splice(index, 0, updated)
  allNotes.update((list) => list.map((n) => (n.id === note.id ? { ...n, items: rest } : n)))
  mutateItem(draggedId, note.id, 'position', position)
}
