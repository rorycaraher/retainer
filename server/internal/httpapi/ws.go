package httpapi

import (
	"net/http"

	"github.com/gorilla/websocket"

	"retainer/server/internal/wsserver"
)

var upgrader = websocket.Upgrader{
	// Single-user, same-origin app served from one Caddy-fronted domain —
	// no cross-origin WS clients are expected, but the default origin check
	// rejects same-origin requests missing an Origin header (e.g. some
	// native/mobile clients later), so this is intentionally permissive.
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleWS(hub *wsserver.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		wsserver.Serve(hub, conn)
	}
}
