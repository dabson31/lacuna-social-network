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

	// how long a deliberate goodbye may wait for the write mutex before we drop the socket
	// anyway. short on purpose: it is a two byte frame, and the goroutine waiting for it is
	// somebody's logout request
	closeWait = 2 * time.Second
)

// ! note: a single tab might be a problem
// gorilla/websocket panics on concurrent writes
// for example, a group message arriving while a notification is being delivered
type Conn struct {
	hub    *Hub
	userID int64
	// sessionID is which login this socket belongs to, not just whose.
	// ! without it the only thing the hub could do on logout is close every socket the user
	// ! has, which would hang up on their other browser too — and two browsers staying
	// ! independent is a thing the audit checks on purpose
	sessionID string
	// expiresAt is the session's own deadline, copied in at the handshake.
	// ? a timestamp and not a session lookup: this package is transport and knows nothing
	// ? about a sessions table. comparing a clock to a number it was handed is still transport
	expiresAt time.Time
	ws        *websocket.Conn
	send      chan []byte

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
		//! here, we're sending a connection into the frame
	case c.send <- frame:
	
	case <-c.done:
		// the case is done so already closed; nothing to deliver to
	default:
		c.log.Warn("ws slow client dropped", "user", c.userID)
		c.close()
	}
}

// ReadMessage in the read pump, which is what triggers the deferred Unregister.
func (c *Conn) close() {
	c.closeWith(websocket.CloseNormalClosure)
}

// closeWith says goodbye with a reason and then drops the socket.
//
// ! the code is the only thing the peer gets to tell the cases apart by, and the difference
// ! decides what the browser does next: 1000 means "the connection ended, reconnect when you
// ! can", 1008 means "the session that authenticated you is gone, stop retrying and find out
// ! who you are". a client that cannot tell them apart either loops forever against a dead
// ! cookie or signs people out over a wifi blip
//
// ! a reason is written HERE rather than left to the write pump, which is what made the code
// ! unreliable: closing the underlying connection in the same breath meant the pump's close
// ! frame raced a socket that was already gone, so the peer usually saw a bare tcp reset —
// ! 1006, abnormal closure, no reason — and every close looked identical from the browser
// ? WriteControl is the one write method gorilla documents as safe to call concurrently with
// ? the write pump: "The Close and WriteControl methods can be called concurrently with all
// ? other methods" (doc.go), and it takes the same write mutex internally, so this is not a
// ? second writer racing the first — which is the thing that panics this library
//
// ! an ordinary close writes nothing here, and that is deliberate rather than lazy. close()
// ! is called from enqueue when a client has fallen too far behind, and enqueue runs on the
// ! goroutine of whoever is fanning a message out — a sender's own http request. WriteControl
// ! waits for the write mutex, which is exactly what a stalled socket is holding, so writing a
// ! goodbye to a client that stopped reading would stall the person who sent the message. the
// ! pump's own best-effort frame still covers that case, as it always did
// ? once.Do means the first reason wins: a socket closed by logout and then by a failed write
// ? keeps the logout reason, which is the one worth reporting
func (c *Conn) closeWith(code int) {
	c.once.Do(func() {
		if c.ws != nil && code != websocket.CloseNormalClosure {
			_ = c.ws.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(code, ""), time.Now().Add(closeWait))
		}
		close(c.done)
		if c.ws != nil {
			_ = c.ws.Close()
		}
	})
}

//! writing to the socket is done here
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
			//! the other half of "a socket must not outlive its login". logout is the common
			//! case and the hub handles it the moment it happens; this is the slow one, a
			//! connection still open when the session's own ttl runs out. one comparison on a
			//! ticker that was already running, and no socket can outlast the session that
			//! authenticated it
			if !c.expiresAt.IsZero() && time.Now().After(c.expiresAt) {
				c.log.Debug("ws session expired", "user", c.userID)
				c.closeWith(websocket.ClosePolicyViolation)
				return
			}

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
			// ? a close with a reason has already sent its own frame, and gorilla records that
			// ? on the connection, so this write returns ErrCloseSent and puts nothing on the
			// ? wire. it is only the ordinary close that still needs it
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
