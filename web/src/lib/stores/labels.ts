import { writable } from 'svelte/store'
import type { Label } from '../types'
import * as api from '../api/rest'
import { stripLabelFromAllNotes } from './notes'

export const labels = writable<Label[]>([])
export const labelsError = writable<string | null>(null)

export async function loadLabels() {
  try {
    const { labels: fetched } = await api.listLabels()
    labels.set([...fetched].sort((a, b) => a.name.localeCompare(b.name)))
    labelsError.set(null)
  } catch (e) {
    labelsError.set(e instanceof Error ? e.message : String(e))
  }
}

export async function createLabel(name: string) {
  const label = await api.createLabel(name)
  labels.update((list) => [...list, label].sort((a, b) => a.name.localeCompare(b.name)))
}

export async function renameLabel(id: string, name: string) {
  const label = await api.renameLabel(id, name)
  labels.update((list) => list.map((l) => (l.id === id ? label : l)).sort((a, b) => a.name.localeCompare(b.name)))
}

// Renaming propagates everywhere automatically: notes reference labels by
// id, so the label store update above is the only client-side state that
// needs refreshing — no per-note bookkeeping to touch.
export async function deleteLabel(id: string) {
  await api.deleteLabel(id)
  labels.update((list) => list.filter((l) => l.id !== id))
  stripLabelFromAllNotes(id)
}
