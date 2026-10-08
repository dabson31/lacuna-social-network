package middleware

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/session"
)

// anything with a ServeHTTP(w, r) method counts as an http.Handler and each middleware is a small struct holding two things
// the settings: a logger, the allowed origin, a size limit; Next: the handler it passes the request on to
//! the structur looks like this: RequestLog → Recover → CORS → MaxBodyBytes → mux → your handler

// so, ResponseWriter lets you only send without reading, statusRecorder saves it
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader is how a handler sends a status code
func (rec *statusRecorder) WriteHeader(code int) {
	if rec.status == 0 {
		rec.status = code
	}
	rec.ResponseWriter.WriteHeader(code)
}

// a handler that calls Write without calling WriteHeader first gets an automatic 200.
// otherwise those requests would be logged as status=0
func (rec *statusRecorder) Write(b []byte) (int, error) {
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	return rec.ResponseWriter.Write(b)
}

// ! hijack is needed for WS
// gorilla/websocket takes over the connection by checking whether the writer has a Hijack method.
// our recorder would hide the real writer's Hijack, and every WebSocket upgrade would fail with "response does not implement http.Hijacker"
func (rec *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rec.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not support hijacking")
	}
	return h.Hijack()
}

// Unwrap returns the real writer, so http.ResponseController can reach its deadline methods through our recorder
func (rec *statusRecorder) Unwrap() http.ResponseWriter { return rec.ResponseWriter }

// RequestLog writes one log line per request: method, path, status, and duration used
type RequestLog struct {
	Log  *slog.Logger
	Next http.Handler
}

func (m RequestLog) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Give everything after us the recorder instead of the real writer,
	// so we can read the status code once they're done.
	rec := &statusRecorder{ResponseWriter: w}

	// Run the rest of the chain (Recover, CORS, ..., the handler).
	// This line only returns once the response has been written.
	m.Next.ServeHTTP(rec, r)

	m.Log.Info("request",
		"method", r.Method,
		"path", r.URL.Path, // only the path, never the query string: queries can carry tokens
		"status", rec.status,
		"duration", time.Since(start),
	)
}

// the catch panic and recover catches any panic in any handler and turns it into a logged 500 json response
// else, net/http catches the panic itself and just cuts the connection, instead of handling it
// ? its like a traceback module in python
// however, this only applies to the request's own goroutine, and not other stuff like a WS RW error
// or even bus handlers: events that listen and execute something right away
type Recover struct {
	Log  *slog.Logger
	Next http.Handler
}

func (m Recover) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// defer makes sure to run this when ServeHTTP exits, regardless of how
	defer m.catchPanic(w, r)
	m.Next.ServeHTTP(w, r)
}

// catchPanic runs at the end of every request
func (m Recover) catchPanic(w http.ResponseWriter, r *http.Request) {
	v := recover()
	if v == nil {
		return
	}

	// here, we let net/http handle it the way it expects to rather than our own way of doing it
	if v == http.ErrAbortHandler {
		panic(v)
	}

	m.Log.Error("panic recovered",
		"err", v,
		"method", r.Method,
		"path", r.URL.Path,
		"stack", string(debug.Stack()),
	)

	// here, we close the connection after this response
	// because reusing the connection could mix those leftover bytes or whatever causes the issue into the next request
	w.Header().Set("Connection", "close")
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
}

// CORS will let the next.js frontend call this API with the session cookie attached
// because the frontend runs on a different origin :3000 vs :8080.
// ! cross-origin resource sharing is a way to loosen the security feature (given by SOP, which is the security feature)
// allowing web apps to share to request resouces from different domains
// basically, browsers follow a rule called `Same-Origin Policy` or SOP. because the browser attaches a request to every site
// so, when you're browsing a.com, SOP makes sure b.com cant read your cookie and requests you've made on a.com.
// a page can't read the responses unless the other server explicitly agrees, and that agreement is the CORS headers
type CORS struct {
	AllowedOrigin string // for example, "http://localhost:3000"
	Next          http.Handler
}

// ! definition and example given by: https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS
// An example of a cross-origin request: the front-end JavaScript code served from
// https://domain-a.com uses fetch() to make a request for https://domain-b.com/data.json.
// For security reasons, browsers restrict cross-origin HTTP requests initiated from scripts.
func (m CORS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// the browser sends the site the request comes from in the Origin header
	origin := r.Header.Get("Origin")
	allowed := origin != "" && origin == m.AllowedOrigin

	// the answer depends on Origin. Tell caches, so they never serve one origin's
	// cached response to another.
	w.Header().Add("Vary", "Origin")

	if allowed {
		// "*" means any origin is allowed
		// apparently,browsers just reject "*" when cookies are sent
		// Echo back the exact origin, never "*": browsers reject "*" when cookies are sent
		w.Header().Set("Access-Control-Allow-Origin", origin)
		// without this the browser drops the session cookie, and every login looks broken

		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// before sending json, the browser first sends a "preflight", which is OPTIONS request to make sure whether its allowed to
	isPreflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""

	if isPreflight {
		if allowed {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			// cache for 10 mins
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// if its a normal reques, it will continue
	m.Next.ServeHTTP(w, r)
}

// MaxBodyBytes simply caps the size of every request body so someone cant send a 5 GB body and fill up memory or disk
type MaxBodyBytes struct {
	Limit int64 // in bytes
	Next  http.Handler
}

// here, we're swapping the body for one that returns an error if more than `Limit` bytes are read
func (m MaxBodyBytes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, m.Limit)
	m.Next.ServeHTTP(w, r)
}

// RequireAuth protects any route that needs a logged-in user.
// it reads the session cookie, looks the session up, and puts the user id in the request context.
// if anything is missing or expired it answers 401 and never calls Next
type RequireAuth struct {
	Sessions *session.Store
	Log      *slog.Logger
	Next     http.Handler
}

func (m RequireAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(session.CookieName)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "you must be logged in")
		return
	}

	sess, err := m.Sessions.Get(r.Context(), cookie.Value)
	if errors.Is(err, session.ErrNotFound) {
		// the browser is holding a dead cookie; tell it to drop it
		m.Sessions.ClearCookie(w)
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "your session has expired")
		return
	}
	if err != nil {
		m.Log.Error("require auth", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	// r.WithContext makes a copy of the request carrying the user id; handlers read it with session.UserID
	ctx := session.WithUserID(r.Context(), sess.UserID)
	m.Next.ServeHTTP(w, r.WithContext(ctx))
}

//! note: this is the mental model:
/*
Browser
   │
   │ HTTP request
   ▼
┌─────────────────┐
│   RequestLog    │  ← starts timer, wraps ResponseWriter
└────────┬────────┘
         ▼
┌─────────────────┐
│    Recover      │  ← catches panics
└────────┬────────┘
         ▼
┌─────────────────┐
│      CORS       │  ← checks browser's Origin
└────────┬────────┘
         ▼
┌─────────────────┐
│ MaxBodyBytes    │  ← limits request body
└────────┬────────┘
         ▼
┌─────────────────┐
│      mux        │  ← finds the route
└────────┬────────┘
         ▼
┌─────────────────┐
│   your handler  │  ← actual application logic
└─────────────────┘
*/
