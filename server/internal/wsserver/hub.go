// Package wsserver is the low-latency "go pull now" signal for connected
// clients: a hub broadcasts a minimal {"type":"changed","serverSeq":N} event
// after every sync write, and clients react by calling POST /api/sync with
// their own sinceSeq. This is the only thing the WS carries — the sync
// endpoint remains the single source of authoritative state, so there's no
// second serialization format to keep in sync with it (see the plan's
// "WebSocket" section).
package wsserver

import (
	"encoding/json"
)

// Event is the wire shape broadcast to every connected client.
type Event struct {
	Type      string `json:"type"`
	ServerSeq int64  `json:"serverSeq"`
}

// Hub tracks connected clients and fans out broadcast events to all of them.
// Its state (the clients map) is only ever touched from the Run loop, so all
// access goes through channels rather than a mutex.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan Event
	clients    map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Event, 16),
		clients:    make(map[*Client]bool),
	}
}

// Run processes register/unregister/broadcast events until stop is closed.
// It must run in its own goroutine for the lifetime of the server.
func (h *Hub) Run(stop <-chan struct{}) {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = true
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
		case ev := <-h.broadcast:
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			for c := range h.clients {
				select {
				case c.send <- payload:
				default:
					// Client's outbound buffer is full (a slow/stuck
					// reader) — drop it rather than block the whole hub.
					delete(h.clients, c)
					close(c.send)
				}
			}
		case <-stop:
			return
		}
	}
}

// Notify broadcasts a "changed" event carrying the new server_seq. It's safe
// to call from any goroutine (e.g. the HTTP handler after a sync write), and
// never blocks: if the hub isn't keeping up (or isn't running), the event is
// dropped — harmless, since it's only a low-latency hint and the next
// successful notify (or a client's own reconnect catch-up) still converges
// clients on the authoritative state.
func (h *Hub) Notify(serverSeq int64) {
	select {
	case h.broadcast <- Event{Type: "changed", ServerSeq: serverSeq}:
	default:
	}
}
