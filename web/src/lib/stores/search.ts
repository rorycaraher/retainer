import { derived, writable } from 'svelte/store'
import type { Note } from '../types'
import * as api from '../api/rest'
import { allNotes } from './notes'

export const searchQuery = writable('')

// null = no active search (show the normal main grid); [] = active search,
// zero matches. Holds just the matching ids — rendering always reads the
// live Note objects out of allNotes below, so a search result card stays
// reactive to concurrent edits/reconciliation instead of going stale.
const matchedIds = writable<string[] | null>(null)

let debounceTimer: ReturnType<typeof setTimeout> | null = null

searchQuery.subscribe((q) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  const trimmed = q.trim()
  if (!trimmed) {
    matchedIds.set(null)
    return
  }
  debounceTimer = setTimeout(async () => {
    try {
      const { notes } = await api.searchNotes(trimmed)
      matchedIds.set(notes.map((n) => n.id))
    } catch {
      matchedIds.set([])
    }
  }, 250)
})

export const searchResults = derived([matchedIds, allNotes], ([ids, all]) => {
  if (ids === null) return null
  const byId = new Map(all.map((n) => [n.id, n]))
  return ids.map((id) => byId.get(id)).filter((n): n is Note => !!n)
})
