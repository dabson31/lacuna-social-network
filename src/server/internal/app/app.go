package app

import (
	"database/sql"
	"log/slog"
	"net/http"
	"social/src/server/internal/app/middleware"
	"social/src/server/internal/platform/config"
	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/session"
	"social/src/server/internal/platform/storage"
)

// this struct contains everything created in main
type Dependencies struct {
	Config config.Config
	DB     *sql.DB
	Log    *slog.Logger
}

// shared holds the platform pieces that are built once and handed to every module
type shared struct {
	sessions    *session.Store
	uploads     *storage.Store
	requireAuth func(http.HandlerFunc) http.Handler
}


// max request body size for an image and whatnot
// for now its 10 mb, but its variable to change
const maxRequestBody = 10 << 20

// this is the only function that knows every module exists
func New(d Dependencies) (http.Handler, error) {

	// the upload store creates the upload dir if it is missing
	uploads, err := storage.New(d.Config.UploadDir)
	if err != nil {
		return nil, err
	}

	// sessions live in the db, the secure cookie flag is only on in production
	sessions := session.NewStore(d.DB, d.Config.SessionTTL, d.Config.Env == "production")

	s := shared{
		sessions: sessions,
		uploads:  uploads,
		// wraps a handler so it only runs for a logged in user
		requireAuth: func(next http.HandlerFunc) http.Handler {
			return middleware.RequireAuth{Sessions: sessions, Log: d.Log, Next: next}
		},
	}

	mux := http.NewServeMux()

	mountHealth(mux, d)
	mountUploads(mux, s)
	mountAuth(mux, d, s)
	mountUsers(mux, d, s)
	mountFollows(mux, d, s)
	mountPosts(mux, d, s)
	mountGroups(mux, d, s)
	mountChat(mux, d, s)
	mountNotifications(mux, d, s)

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
	handler := middleware.RequestLog{
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
	return handler, nil
}
