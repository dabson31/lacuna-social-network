package app

import (
	"context"
	"net/http"
	"time"

	"social/src/server/internal/platform/httpx"
)

// Health lives in app/, not in a module: it has no domain logic, it just
// checks that the process and the DB are reachable.
func mountHealth(mux *http.ServeMux, d Dependencies) {
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		// Ping the DB for real. A health check that returns "ok" while the database is
		// unreachable is worse than having none: Docker will think the service is fine.
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.DB.PingContext(ctx); err != nil {
			httpx.WriteError(w, http.StatusServiceUnavailable, "db_unavailable", "database is unreachable")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

// One mount function per module. Each phase fills in exactly one of these and nothing else
// in this file changes. Right now they're empty, so the list is also your progress tracker.
func mountAuth(mux *http.ServeMux, d Dependencies)          {} // Phase 3
func mountUsers(mux *http.ServeMux, d Dependencies)         {} // Phase 4
func mountFollows(mux *http.ServeMux, d Dependencies)       {} // Phase 4
func mountPosts(mux *http.ServeMux, d Dependencies)         {} // Phase 5
func mountGroups(mux *http.ServeMux, d Dependencies)        {} // Phase 6
func mountChat(mux *http.ServeMux, d Dependencies)          {} // Phase 7
func mountNotifications(mux *http.ServeMux, d Dependencies) {} // Phase 8
