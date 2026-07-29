// The WS connection is purely a low-latency "go pull now" signal — on any
// "changed" message we just trigger a normal /api/sync catch-up pull via
// syncNow(). There's no second serialization format to keep in sync with
// the REST/sync protocol.
import { writable } from 'svelte/store'
import { syncNow } from '../stores/notes'

export const wsConnected = writable(false)

let socket: WebSocket | null = null
let stopped = true
let reconnectDelay = 1000
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let pullTimer: ReturnType<typeof setTimeout> | null = null

const MAX_RECONNECT_DELAY = 30000
const PULL_DEBOUNCE_MS = 150

function schedulePull() {
  if (pullTimer) clearTimeout(pullTimer)
  pullTimer = setTimeout(syncNow, PULL_DEBOUNCE_MS)
}

function connect() {
  if (stopped) return
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  socket = new WebSocket(`${proto}//${location.host}/ws`)

  socket.onopen = () => {
    wsConnected.set(true)
    reconnectDelay = 1000
    syncNow() // catch-up pull: covers "was offline," "server restarted," "missed an event" alike
  }

  socket.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'changed') schedulePull()
    } catch {
      // ignore malformed frames
    }
  }

  socket.onclose = () => {
    wsConnected.set(false)
    if (stopped) return
    reconnectTimer = setTimeout(connect, reconnectDelay)
    reconnectDelay = Math.min(reconnectDelay * 2, MAX_RECONNECT_DELAY)
  }

  socket.onerror = () => {
    socket?.close()
  }
}

export function startWSClient() {
  if (!stopped) return
  stopped = false
  reconnectDelay = 1000
  connect()
}

export function stopWSClient() {
  stopped = true
  if (reconnectTimer) clearTimeout(reconnectTimer)
  if (pullTimer) clearTimeout(pullTimer)
  socket?.close()
  socket = null
  wsConnected.set(false)
}
