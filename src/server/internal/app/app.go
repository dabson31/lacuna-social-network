package app

import (
	"database/sql"
	"log/slog"
	"net/http"
	"social/src/server/internal/app/middleware"
	"social/src/server/internal/platform/config"
	"social/src/server/internal/platform/httpx"
)

// this struct contains everything created in main
type Dependencies struct {
	Config config.Config
	DB     *sql.DB
	Log    *slog.Logger
}

// max request body size for an image and whatnot
// for now its 10 mb, but its variable to change
const maxRequestBody = 10 << 20

// this is the only function that knows every module exists
func New(d Dependencies) http.Handler {

	mux := http.NewServeMux()

	// mountHealth(mux, d)
	// mountAuth(mux, d)
	// mountUser(mux, d)
	// mountFollows(mux, d)
	// mountPosts(mux, d)
	// mountGrops(mux, d)
	// mountChat(mux, d)
	// mountNotification(mux, d)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "No such endpoint was found")
	})

	//todo
	// wrap the router in middleware itself; each one's Next is the one indented under it
	// meaning a request runs through them top to bottom and reaches the mux last.
	// the architectural choice behind the choice matters here:
	// 1. RequestLog is first, so it logs the final status
	// 2. Recover is second to catch a panic in anything below it
	// 3. CORS comes before the mux, this way, the preflights are answered before the router can 405 them
	return middleware.RequestLog{
		Log: d.Log,
		Next: middleware.Recover{
			Log: d.Log,
			Next: middleware.CORS{
				AllowedOrigin: d.Config.AllowedOrigin,
				Next: middleware.MaxBodyBytes{
					Limit: maxRequestBody,
					Next:  mux,
				},
			},
		},
	}
}
