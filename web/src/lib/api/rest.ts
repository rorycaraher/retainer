import type { ChecklistItem, Label, Note } from '../types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`${init?.method ?? 'GET'} ${path} failed: ${res.status} ${body}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export function listNotes(): Promise<{ notes: Note[] }> {
  return request('/api/notes')
}

export interface Mutation {
  entity: 'note' | 'item' | 'note_label'
  id?: string
  noteId?: string
  labelId?: string
  field: string
  value: unknown
  hlc: string
}

export interface NoteLabelChange {
  noteId: string
  labelId: string
  attached: boolean
  serverSeq: number
}

export interface SyncChanges {
  notes: Note[]
  items: ChecklistItem[]
  noteLabels: NoteLabelChange[]
}

export interface SyncResponse {
  serverSeq: number
  changes: SyncChanges
}

export function sync(sinceSeq: number, mutations: Mutation[]): Promise<SyncResponse> {
  return request('/api/sync', {
    method: 'POST',
    body: JSON.stringify({ sinceSeq, mutations }),
  })
}

// purgeNote is the explicit "delete forever" action from Trash: immediate
// and irreversible, deliberately outside the generic sync mutation batch.
export function purgeNote(id: string): Promise<void> {
  return request(`/api/notes/${id}/purge`, { method: 'DELETE' })
}

export function searchNotes(q: string): Promise<{ notes: Note[] }> {
  return request(`/api/notes/search?q=${encodeURIComponent(q)}`)
}

export function listLabels(): Promise<{ labels: Label[] }> {
  return request('/api/labels')
}

export function createLabel(name: string): Promise<Label> {
  return request('/api/labels', { method: 'POST', body: JSON.stringify({ name }) })
}

export function renameLabel(id: string, name: string): Promise<Label> {
  return request(`/api/labels/${id}`, { method: 'PATCH', body: JSON.stringify({ name }) })
}

export function deleteLabel(id: string): Promise<void> {
  return request(`/api/labels/${id}`, { method: 'DELETE' })
}

export async function login(password: string): Promise<void> {
  const res = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  })
  if (!res.ok) {
    if (res.status === 401) throw new Error('Incorrect password')
    if (res.status === 429) throw new Error('Too many attempts, try again later')
    throw new Error(`Login failed: ${res.status}`)
  }
}

export function logout(): Promise<void> {
  return request('/api/auth/logout', { method: 'POST', body: JSON.stringify({}) })
}

export async function authStatus(): Promise<boolean> {
  const res = await fetch('/api/auth/status')
  return res.ok
}
