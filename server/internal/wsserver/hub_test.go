package wsserver

import (
	"encoding/json"
	"testing"
	"time"
)

func startHub(t *testing.T) *Hub {
	t.Helper()
	h := NewHub()
	stop := make(chan struct{})
	go h.Run(stop)
	t.Cleanup(func() { close(stop) })
	return h
}

func newTestClient(h *Hub) *Client {
	return &Client{hub: h, send: make(chan []byte, sendBuffer)}
}

func recvEvent(t *testing.T, c *Client) Event {
	t.Helper()
	select {
	case msg, ok := <-c.send:
		if !ok {
			t.Fatal("client channel closed unexpectedly")
		}
		var ev Event
		if err := json.Unmarshal(msg, &ev); err != nil {
			t.Fatalf("unmarshal event: %v", err)
		}
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for broadcast")
		return Event{}
	}
}

func TestHubBroadcastsToRegisteredClients(t *testing.T) {
	h := startHub(t)
	a := newTestClient(h)
	b := newTestClient(h)
	h.register <- a
	h.register <- b

	h.Notify(42)

	evA := recvEvent(t, a)
	evB := recvEvent(t, b)
	if evA.Type != "changed" || evA.ServerSeq != 42 {
		t.Fatalf("unexpected event for client A: %+v", evA)
	}
	if evB.Type != "changed" || evB.ServerSeq != 42 {
		t.Fatalf("unexpected event for client B: %+v", evB)
	}
}

func TestHubUnregisterStopsDeliveryAndClosesChannel(t *testing.T) {
	h := startHub(t)
	a := newTestClient(h)
	h.register <- a
	h.unregister <- a

	// Give the hub loop a moment to process the unregister before asserting.
	time.Sleep(50 * time.Millisecond)

	h.Notify(1)

	select {
	case _, ok := <-a.send:
		if ok {
			t.Fatal("expected no broadcast to reach an unregistered client")
		}
		// ok == false means the channel was closed by unregister, as expected.
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected unregistered client's channel to be closed")
	}
}

func TestHubNotifyWithNoClientsDoesNotBlock(t *testing.T) {
	h := startHub(t)
	done := make(chan struct{})
	go func() {
		h.Notify(1)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Notify blocked with no registered clients")
	}
}

func TestHubDropsSlowClientWithoutBlockingOthers(t *testing.T) {
	h := startHub(t)
	slow := newTestClient(h) // never drained, buffer will fill
	healthy := newTestClient(h)
	h.register <- slow
	h.register <- healthy

	// Fill the slow client's buffer, then send one more to force the hub to
	// drop it (see Hub.Run's broadcast case: full buffer -> unregister).
	for i := 0; i < sendBuffer+1; i++ {
		h.Notify(int64(i))
		// Drain the healthy client each time so it never itself blocks the
		// assertion below on its own backpressure.
		select {
		case <-healthy.send:
		case <-time.After(2 * time.Second):
			t.Fatalf("healthy client did not receive broadcast %d", i)
		}
	}

	// slow.send still has its buffered-but-undelivered events queued; drain
	// those first — the closed signal only surfaces once they're exhausted.
	closed := false
	for i := 0; i < sendBuffer+5 && !closed; i++ {
		select {
		case _, ok := <-slow.send:
			closed = !ok
		case <-time.After(2 * time.Second):
			t.Fatal("timed out draining slow client's channel")
		}
	}
	if !closed {
		t.Fatal("expected slow client's channel to eventually close after being dropped")
	}
}
