package app

import (
	"context"
	"net/http"
	"time"

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

// because everything is modular, there is one function per module
func mountAuth(mux *http.ServeMux, d Dependencies)          {} 
func mountUsers(mux *http.ServeMux, d Dependencies)         {} 
func mountFollows(mux *http.ServeMux, d Dependencies)       {} 
func mountPosts(mux *http.ServeMux, d Dependencies)         {} 
func mountGroups(mux *http.ServeMux, d Dependencies)        {} 
func mountChat(mux *http.ServeMux, d Dependencies)          {} 
func mountNotifications(mux *http.ServeMux, d Dependencies) {} 
