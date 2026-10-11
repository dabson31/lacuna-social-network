package realtime

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// how long a single write may take before we give up on the client
	writeWait = 10 * time.Second

	// a client that has not answered a ping in this long is treated as gone.
	// for example, half-open connections like  laptop lid closed
	pongWait = 60 * time.Second

	// ping more often than pongWait so one lost ping is not fatal
	pingPeriod = (pongWait * 9) / 10

	// refuse anything larger instead of allocating it
	readLimit = 4 << 10

	// how many frames may be in flight for one socket before we call it stalled
	sendBuffer = 32

	// a frame handler touches the DB, so it gets a deadline of its own
	frameTimeout = 5 * time.Second
)

// ! note: a single tab might be a problem
// gorilla/websocket panics on concurrent writes, and
// for example, a group message arriving while a notification is being delivered
type Conn struct {
	hub    *Hub
	userID int64
	ws     *websocket.Conn
	send   chan []byte

	// a send on a closed channel panics, enqueue can be called from any number of goroutines at once
	// so no moment at which closing it would be safe.
	done chan struct{}
	once sync.Once

	log *slog.Logger
}

func (c *Conn) UserID() int64 { return c.userID }

//! geeksforgeeks:
// Select in Go allows a goroutine to wait on multiple channel operations, enabling it to choose one that is ready to proceed.
// It simplifies handling multiple channels by blocking until one of the cases can run, making concurrent programming more efficient.

// eif a client is so far behind that sendBuffer frames are already queued
// a dropped message is recoverable, the row is in SQLite and the next fetch gets it
// but cant risk a blocked hub
func (c *Conn) enqueue(frame []byte) {
	select {
	case c.send <- frame:
	case <-c.done:
		// already closed; nothing to deliver to
	default:
		c.log.Warn("ws slow client dropped", "user", c.userID)
		c.close()
	}
}

// ReadMessage in the read pump, which is what triggers the deferred Unregister.
func (c *Conn) close() {
	c.once.Do(func() {
		close(c.done)
		if c.ws != nil {
			_ = c.ws.Close()
		}
	})
}

// writePump is the only goroutine that writes to this socket, ever.
func (c *Conn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case frame := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.TextMessage, frame); err != nil {
				// a failed write means the socket is gone; close and let the read pump,
				// which is about to fail too, do the unregistering
				c.close()
				return
			}

		case <-ticker.C:
			// the keepalive. Its real job is not liveness but the read deadline on the
			// other side: no ping, no pong, and pongWait elapses on a connection that
			// looks perfectly healthy from here.
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.close()
				return
			}

		case <-c.done:
			// best effort close handshake, so the browser sees a clean close and its
			// reconnect logic can tell "server said goodbye" from "network died"
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			_ = c.ws.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return
		}
	}
}

// readPump is the only goroutine that reads. Its visible job is routing inbound frames;
// its real job is keeping the connection honest and guaranteeing Unregister runs however
// the connection ends.
func (c *Conn) readPump(onFrame func(context.Context, Envelope)) {
	defer func() {
		c.hub.Unregister(c)
		c.close()
	}()

	c.ws.SetReadLimit(readLimit)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))

	// every pong pushes the deadline out. Stop hearing pongs and the next read fails on
	// its own — that failure IS the half-open-connection collector.
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.ws.ReadMessage()
		if err != nil {
			// a tab closing is normal and not worth an error line; anything else might be
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				c.log.Debug("ws read ended", "user", c.userID, "err", err)
			}
			return
		}

		env, err := Decode(raw)
		if err != nil {
			// garbage from one client is not a reason to hang up on it
			c.log.Debug("ws undecodable frame", "user", c.userID, "err", err)
			continue
		}

		// a fresh context, not the request's: handlers here outlive any single request,
		// and a frame handler that queries the DB needs a deadline of its own
		ctx, cancel := context.WithTimeout(context.Background(), frameTimeout)
		onFrame(ctx, env)
		cancel()
	}
}
