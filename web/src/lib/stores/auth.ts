import { writable } from 'svelte/store'
import * as api from '../api/rest'
import { startWSClient, stopWSClient } from '../ws/client'

export const isAuthed = writable<boolean | null>(null) // null = not yet checked

export async function checkAuth() {
  const authed = await api.authStatus()
  isAuthed.set(authed)
  if (authed) startWSClient()
}

export async function login(password: string) {
  await api.login(password)
  isAuthed.set(true)
  startWSClient()
}

export async function logout() {
  await api.logout()
  isAuthed.set(false)
  stopWSClient()
}
