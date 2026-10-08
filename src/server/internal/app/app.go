package app

import (
	"database/sql"
	"log/slog"
	"net/http"
	"social/src/server/internal/app/middleware"
	"social/src/server/internal/platform/config"
	"social/src/server/internal/platform/events"
	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/realtime"
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
	sessions *session.Store
	uploads  *storage.Store
	// bus is the in process pub sub, features publish to it and notifications listens
	bus *events.Bus
	// hub is the websocket connection registry. it is built here like the bus because two
	// unrelated features push through it, chat and notifications, and neither can own it
	hub         *realtime.Hub
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

	// the event bus lets features announce things without importing each other, follows
	// and groups publish, notifications listens, it is created here and handed to shared
	bus := events.NewBus(d.Log)

	// the hub holds every open socket in memory, keyed by user id. one per process, same
	// as the bus, and for the same reason: it is state that the modules borrow
	hub := realtime.NewHub(d.Log)

	s := shared{
		sessions: sessions,
		uploads:  uploads,
		bus:      bus,
		hub:      hub,
		// wraps a handler so it only runs for a logged in user
		requireAuth: func(next http.HandlerFunc) http.Handler {
			return middleware.RequireAuth{Sessions: sessions, Log: d.Log, Next: next}
		},
	}

	mux := http.NewServeMux()

	mountHealth(mux, d)
	mountUploads(mux, s)
	mountAuth(mux, d, s)

	//! chat needs three facts it doesnt own: the follow relation, whether a profile is
	//! public, and group membership. each one is a tiny interface declared over in chat and
	//! satisfied by whoever owns the data, which is why these three mounts hand their
	//! service back now instead of dropping it. this is the composition root doing the one
	//! job only it can do, introducing modules that arent allowed to import each other
	usersSvc := mountUsers(mux, d, s)
	followsSvc := mountFollows(mux, d, s)
	mountPosts(mux, d, s)
	groupsSvc := mountGroups(mux, d, s)

	chatSvc := mountChat(mux, d, s, followsSvc, usersSvc, groupsSvc)
	mountNotifications(mux, d, s)

	// the socket goes on last because it needs the chat service to route inbound frames
	mountRealtime(mux, d, s, chatSvc)

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
