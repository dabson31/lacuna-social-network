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
	DB 	*sql.DB
	Log	*slog.Logger
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
	return middleware.Chain(mux, 
		middleware.RequestLog(d.Log),
		middleware.Recover(d.Log),
		middleware.CORS(d.Config.AllowedOrigin),
		middleware.MaxBodyBytes(maxRequestBody),
	)
}