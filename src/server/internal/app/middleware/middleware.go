package middleware

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"social-network/internal/platform/httpx"
	"time"
)

type Middleware func(http.Handler) http.Handler

// Chain (h, A, B, C) == A(B(C(h))), basically starts with handler h, wraps C around it, then wraps B around C(h), and then A around B(C(h))
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// statusRecorder wraps response writer  to remember the statusd code because it has no way to ask what the handler sent
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

// if a handler calls write with writeheader, and gets an implicit 200 we need to record that else the reqs will be logged as status=0
func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// we need this for the websocket upgrade from normal http, as without it every websocket upgrade would fail with "response does not implement http.Hijacker"
// so we need to just forward it here so it doesnt cause said issues
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not support hijacking")
	}
	return h.Hijack()
}

// unwrap to let http response controller reach th real writer (flush, deadlines)
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// log slog 🗣️
func RequestLog(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration", time.Since(start),
			)
		})
	}
}

// this is our defence against panics, we turn them into a logged 500 instead of dropping connection
// so net/http does not exactl recover panics itself, instead it cuts the connection, the browser sees
// "network error" with no json and the stack goes to stderr instead of the logger.
// (this will only work for the request's own goroutine, a panic in websockets read/write pumps and
// event bus handlers WILL crash the whole process, and this middle wouldnt be able to catch it)
func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if v == http.ErrAbortHandler {
					panic(v)
				}

				log.Error("panic recovered successfully",
					"err", v,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				// close connection after response cause handler mightve left the reuest body half read, and reusing this
				// connection could ruin the next request
				w.Header().Set("Connection", "close")
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			}()
			next.ServeHTTP(w, r)
		})

	}
}

// CORS lets the Next.js origin call this API WITH cookies.
func CORS(allowedOrigin string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := origin != "" && origin == allowedOrigin

			w.Header().Add("Vary", "Origin")
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if allowed {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
					// Browsers cache the preflight for 10 minutes instead of doubling every request.
					w.Header().Set("Access-Control-Max-Age", "600")
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// this is the global upper limit that applies before ANY handler reads the body
// you can make it smaller like in DecodeJSON, but you cant loosen it. (5GB LIMIT)
func MaxBodyBytes(n int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, http.StatusNotImplemented, "not_implement", "authentication is not implemented yet")
	})
}
