package app

import (
	"context"
	"net/http"
	"time"

	"social/src/server/internal/modules/auth"
	"social/src/server/internal/platform/httpx"
)

// the health is in app/ and not in a module because it just checks that the process and the DB are reachable
func mountHealth(mux *http.ServeMux, d Dependencies) {
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		// ping the DB and check whether it returns anything
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.DB.PingContext(ctx); err != nil {
			httpx.WriteError(w, http.StatusServiceUnavailable, "db_unavailable", "database is unreachable")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

// serves an uploaded image by its file name, the wildcard is a single path segment so it cannot walk into other folders
func mountUploads(mux *http.ServeMux, s shared) {
	mux.HandleFunc("GET /uploads/{name}", s.uploads.Serve)
}

// because everything is modular, there is one function per module
// builds the auth store service and handler then mounts its routes, logout and me sit behind requireAuth
func mountAuth(mux *http.ServeMux, d Dependencies, s shared) {
	store := auth.NewStore(d.DB)
	svc := auth.NewService(store, s.sessions, s.uploads)
	h := auth.NewHandler(svc, s.sessions, d.Log)

	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.Handle("POST /api/auth/logout", s.requireAuth(h.Logout))
	mux.Handle("GET /api/auth/me", s.requireAuth(h.Me))
}

func mountUsers(mux *http.ServeMux, d Dependencies, s shared)         {}
func mountFollows(mux *http.ServeMux, d Dependencies, s shared)       {}
func mountPosts(mux *http.ServeMux, d Dependencies, s shared)         {}
func mountGroups(mux *http.ServeMux, d Dependencies, s shared)        {}
func mountChat(mux *http.ServeMux, d Dependencies, s shared)          {}
func mountNotifications(mux *http.ServeMux, d Dependencies, s shared) {}
