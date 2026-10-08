package realtime

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"

	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/session"
)

// Handler is the seam between transport and domain: the gateway hands a decoded frame and
// the user it came from to whoever was wired in, and knows nothing about what happens next.
type Handler func(ctx context.Context, userID int64, env Envelope)

// Gateway is the HTTP end of the socket — handshake, upgrade, start the pumps, register.
// It is deliberately the only file in this package that imports net/http.
type Gateway struct {
	hub      *Hub
	log      *slog.Logger
	upgrader websocket.Upgrader
	onFrame  Handler
}

// NewGateway builds the gateway. onFrame may be nil for a push-only socket.
func NewGateway(hub *Hub, log *slog.Logger, allowedOrigin string, onFrame Handler) *Gateway {
	return &Gateway{
		hub:     hub,
		log:     log,
		onFrame: onFrame,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,

			// gorilla's default CheckOrigin returns true for everything, which is CSRF over
			// a socket: any page on the internet could open a ws to us and the browser
			// would attach the session cookie, because the WebSocket handshake is not
			// subject to the same-origin policy the way fetch is. Our CORS middleware does
			// not help — it guards fetch, not this.
			//
			// An empty Origin is allowed on purpose: non-browser clients (websocat, the
			// tests below) send none, and a browser always sends one.
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "" || origin == allowedOrigin
			},
		},
	}
}

// ServeWS is mounted at GET /ws behind RequireAuth.
//
// Authentication happens before the upgrade, which is the whole point: the session cookie
// rides along on the handshake like on any other GET, so this is an ordinary middleware
// check. No token in the query string — those end up in access logs, browser history and
// Referer headers.
func (g *Gateway) ServeWS(w http.ResponseWriter, r *http.Request) {
	userID, ok := session.UserID(r.Context())
	if !ok {
		// unreachable behind RequireAuth, but a socket handler that quietly accepts
		// anonymous connections is a bug worth making structurally impossible
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "you must be logged in")
		return
	}

	ws, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade has already written its own error response, so we only log
		g.log.Warn("ws upgrade failed", "user", userID, "err", err)
		return
	}

	c := &Conn{
		hub:    g.hub,
		userID: userID,
		ws:     ws,
		send:   make(chan []byte, sendBuffer),
		done:   make(chan struct{}),
		log:    g.log,
	}
	g.hub.Register(c)

	go c.writePump()

	// the read pump runs on the request goroutine rather than a third one: this handler
	// then stays alive for the life of the socket, which is the simplest correct lifetime.
	// It also means RequestLog writes its line when the socket closes, with duration equal
	// to the whole session — expected, not a bug.
	c.readPump(g.frameHandler(c))
}

// frameHandler builds this connection's inbound dispatcher.
//
// Transport frames are answered here; everything else goes to the feature handler under a
// recover. The Recover middleware cannot help us: by the time a frame arrives the response
// has long been hijacked, so a panic would reach net/http with nothing to write to and take
// the process down. One bad frame must cost one socket, not the server.
func (g *Gateway) frameHandler(c *Conn) func(context.Context, Envelope) {
	return func(ctx context.Context, env Envelope) {
		// application-level ping, because the browser's WebSocket API cannot send a
		// protocol-level one. A client that wants to check its own connection has no other
		// option, so this is not redundant with the ping in writePump — it is the mirror.
		if env.Type == TypePing {
			if frame, err := Encode(TypePong, nil); err == nil {
				c.enqueue(frame)
			}
			return
		}

		if g.onFrame == nil {
			return
		}
		defer func() {
			if v := recover(); v != nil {
				g.log.Error("ws frame handler panic", "user", c.userID, "type", env.Type, "err", v)
			}
		}()
		g.onFrame(ctx, c.userID, env)
	}
}
