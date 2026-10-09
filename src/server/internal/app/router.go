package app

import (
	"context"
	"net/http"
	"time"

	"social/src/server/internal/modules/auth"
	"social/src/server/internal/modules/chat"
	"social/src/server/internal/modules/follows"
	"social/src/server/internal/modules/groups"
	"social/src/server/internal/modules/notifications"
	"social/src/server/internal/modules/posts"
	"social/src/server/internal/modules/users"
	"social/src/server/internal/platform/events"
	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/realtime"
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

// builds the users store service and handler then mounts the profile read, the two self
// only patches and the browse search, every route needs a logged in caller
func mountUsers(mux *http.ServeMux, d Dependencies, s shared) *users.Service {
	store := users.NewStore(d.DB)
	svc := users.NewService(store)
	h := users.NewHandler(svc, d.Log)

	mux.Handle("GET /api/users", s.requireAuth(h.Search))
	mux.Handle("GET /api/users/{id}", s.requireAuth(h.Get))
	mux.Handle("PATCH /api/users/me", s.requireAuth(h.UpdateMe))
	mux.Handle("PATCH /api/users/me/privacy", s.requireAuth(h.SetPrivacy))
	// handed back so chat can use it as its ProfileChecker
	return svc
}

// follows owns the follow graph, its routes hang off a user and off follow-requests, the
// bus is set here so a request to a private account turns into a notification
func mountFollows(mux *http.ServeMux, d Dependencies, s shared) *follows.Service {
	store := follows.NewStore(d.DB)
	svc := follows.NewService(store)
	svc.Bus = s.bus
	h := follows.NewHandler(svc, d.Log)

	mux.Handle("POST /api/users/{id}/follow", s.requireAuth(h.Follow))
	mux.Handle("DELETE /api/users/{id}/follow", s.requireAuth(h.Unfollow))
	mux.Handle("GET /api/users/{id}/followers", s.requireAuth(h.Followers))
	mux.Handle("GET /api/users/{id}/following", s.requireAuth(h.Following))

	mux.Handle("GET /api/follow-requests", s.requireAuth(h.Incoming))
	mux.Handle("POST /api/follow-requests/{id}/accept", s.requireAuth(h.Accept))
	mux.Handle("POST /api/follow-requests/{id}/decline", s.requireAuth(h.Decline))
	// handed back so chat can use it as its FollowChecker
	return svc
}

// posts owns the feed, the three privacy levels and comments. it needs no bus and no ports,
// it does its own visibility sql inside its store, so this mount is just routes
//! GET /api/users/{id}/posts lives here and not in mountUsers even though the path hangs off
//! a user, because the thing being returned is posts and the visibility rule deciding what
//! comes back is the posts module's rule
func mountPosts(mux *http.ServeMux, d Dependencies, s shared) {
	store := posts.NewStore(d.DB)
	svc := posts.NewService(store, s.uploads)
	h := posts.NewHandler(svc, d.Log)

	mux.Handle("POST /api/posts", s.requireAuth(h.Create))
	mux.Handle("GET /api/posts", s.requireAuth(h.Feed))
	mux.Handle("GET /api/posts/{id}", s.requireAuth(h.Get))
	mux.Handle("DELETE /api/posts/{id}", s.requireAuth(h.Delete))
	mux.Handle("GET /api/users/{id}/posts", s.requireAuth(h.ByAuthor))

	mux.Handle("POST /api/posts/{id}/comments", s.requireAuth(h.CreateComment))
	mux.Handle("GET /api/posts/{id}/comments", s.requireAuth(h.Comments))
}

// groups is the largest feature, it mounts the lifecycle, the two ways in by invite and by
// request, the members only content and events, and the event respond route which hangs off
// events not groups, the bus is set so invites join requests and new events become notifications
func mountGroups(mux *http.ServeMux, d Dependencies, s shared) *groups.Service {
	store := groups.NewStore(d.DB)
	svc := groups.NewService(store, s.uploads)
	svc.Bus = s.bus
	h := groups.NewHandler(svc, d.Log)

	mux.Handle("POST /api/groups", s.requireAuth(h.Create))
	mux.Handle("GET /api/groups", s.requireAuth(h.Browse))
	mux.Handle("GET /api/groups/{id}", s.requireAuth(h.Get))

	mux.Handle("POST /api/groups/{id}/invite", s.requireAuth(h.Invite))
	mux.Handle("POST /api/groups/{id}/invitations/accept", s.requireAuth(h.AcceptInvite))
	mux.Handle("POST /api/groups/{id}/invitations/decline", s.requireAuth(h.DeclineInvite))
	mux.Handle("GET /api/invitations", s.requireAuth(h.Invitations))

	mux.Handle("POST /api/groups/{id}/join", s.requireAuth(h.Join))
	mux.Handle("GET /api/groups/{id}/requests", s.requireAuth(h.Requests))
	mux.Handle("POST /api/groups/{id}/requests/{userID}/accept", s.requireAuth(h.AcceptRequest))
	mux.Handle("POST /api/groups/{id}/requests/{userID}/decline", s.requireAuth(h.DeclineRequest))
	mux.Handle("DELETE /api/groups/{id}/membership", s.requireAuth(h.Leave))

	mux.Handle("POST /api/groups/{id}/posts", s.requireAuth(h.CreatePost))
	mux.Handle("GET /api/groups/{id}/posts", s.requireAuth(h.Posts))
	mux.Handle("POST /api/groups/{id}/posts/{postID}/comments", s.requireAuth(h.CreateComment))
	mux.Handle("GET /api/groups/{id}/posts/{postID}/comments", s.requireAuth(h.Comments))

	mux.Handle("POST /api/groups/{id}/events", s.requireAuth(h.CreateEvent))
	mux.Handle("GET /api/groups/{id}/events", s.requireAuth(h.Events))
	mux.Handle("POST /api/events/{id}/respond", s.requireAuth(h.Respond))
	// handed back so chat can use it as its GroupChecker
	return svc
}

// chat owns both halves of messaging, dms and group rooms. the three services arrive as the
// narrow interfaces chat declared for itself, so this function is the only place in the
// whole codebase that knows chat and follows are related at all
func mountChat(mux *http.ServeMux, d Dependencies, s shared,
	followsSvc chat.FollowChecker, usersSvc chat.ProfileChecker, groupsSvc chat.GroupChecker) *chat.Service {

	store := chat.NewStore(d.DB)
	svc := chat.NewService(store, followsSvc, usersSvc, groupsSvc, d.Log)
	// the hub goes in as a chat.Pusher, so the service can fan out and literally nothing
	// else, it cant register, unregister or close a connection even by mistake
	svc.Push = s.hub
	h := chat.NewHandler(svc, d.Log)

	mux.Handle("POST /api/messages", s.requireAuth(h.SendDM))
	mux.Handle("GET /api/messages/{userID}", s.requireAuth(h.DMHistory))
	mux.Handle("POST /api/messages/{userID}/read", s.requireAuth(h.MarkDMRead))
	mux.Handle("GET /api/conversations", s.requireAuth(h.Inbox))

	mux.Handle("POST /api/groups/{id}/messages", s.requireAuth(h.SendGroupMessage))
	mux.Handle("GET /api/groups/{id}/messages", s.requireAuth(h.GroupHistory))
	mux.Handle("POST /api/groups/{id}/messages/read", s.requireAuth(h.MarkGroupRead))

	return svc
}

// mountRealtime puts the one socket behind requireAuth, so the session gets checked on the
// handshake before any upgrade happens. the cookie rides along on a GET like it does
// everywhere else, which is why no token in the query string is needed
//! /ws sits outside /api on purpose, its not a REST resource, and the catch all 404 on
//! /api/ in app.go would otherwise have an opinion about it
func mountRealtime(mux *http.ServeMux, d Dependencies, s shared, chatSvc *chat.Service) {
	gw := realtime.NewGateway(s.hub, d.Log, d.Config.AllowedOrigin, chatSvc.Receive)
	mux.Handle("GET /ws", s.requireAuth(gw.ServeWS))
}

// notifications is the one place that couples the four event kinds to the recorder, every
// kind writes a row through the same record, the publish is fire and forget so its error
// is dropped here, the group kinds are subscribed now and stay idle until groups publishes them
func mountNotifications(mux *http.ServeMux, d Dependencies, s shared) {
	store := notifications.NewStore(d.DB)
	svc := notifications.NewService(store, d.Log)
	// with the hub wired, every one of the four kinds becomes a stored row AND a live
	// notification.new frame. this is the half of phase 8 that was waiting on the socket
	svc.Push = s.hub
	h := notifications.NewHandler(svc, d.Log)

	record := func(ctx context.Context, e events.Event) { _ = svc.Record(ctx, e) }
	s.bus.Subscribe(events.FollowRequested, record)
	s.bus.Subscribe(events.GroupInvited, record)
	s.bus.Subscribe(events.GroupJoinRequested, record)
	s.bus.Subscribe(events.EventCreated, record)

	mux.Handle("GET /api/notifications", s.requireAuth(h.List))
	mux.Handle("POST /api/notifications/{id}/read", s.requireAuth(h.MarkRead))
	mux.Handle("POST /api/notifications/read-all", s.requireAuth(h.MarkAllRead))
}
