package follows

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/session"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// POST /api/users/{id}/follow
func (h *Handler) Follow(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	targetID, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "user id must be a number")
		return
	}
	res, err := h.svc.Follow(r.Context(), viewerID, targetID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// DELETE /api/users/{id}/follow
func (h *Handler) Unfollow(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	targetID, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "user id must be a number")
		return
	}
	if err := h.svc.Unfollow(r.Context(), viewerID, targetID); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/follow-requests/{id}/accept
func (h *Handler) Accept(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, true)
}

// POST /api/follow-requests/{id}/decline
func (h *Handler) Decline(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, false)
}

// decide is the shared body of accept and decline, they differ by one call
func (h *Handler) decide(w http.ResponseWriter, r *http.Request, accept bool) {
	viewerID, _ := session.UserID(r.Context())
	followID, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "request id must be a number")
		return
	}
	if accept {
		err = h.svc.Accept(r.Context(), viewerID, followID)
	} else {
		err = h.svc.Decline(r.Context(), viewerID, followID)
	}
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/follow-requests
func (h *Handler) Incoming(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	list, err := h.svc.IncomingRequests(r.Context(), viewerID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []RequestItem{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// GET /api/users/{id}/followers
func (h *Handler) Followers(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Followers)
}

// GET /api/users/{id}/following
func (h *Handler) Following(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Following)
}

// list is the shared body of the followers and following handlers, fn is whichever
// service call applies, both take the viewer and the owner and return the same shape
func (h *Handler) list(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64) ([]UserCard, error)) {
	viewerID, _ := session.UserID(r.Context())
	ownerID, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "user id must be a number")
		return
	}
	list, err := fn(r.Context(), viewerID, ownerID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []UserCard{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errSelf):
		httpx.WriteError(w, http.StatusBadRequest, "self_follow", "you cannot follow yourself")
	case errors.Is(err, errUserMissing), errors.Is(err, errNoRequest):
		// a missing user and a request you may not touch both read as not found, we never
		// confirm that a private thing exists to someone not allowed to see it
		httpx.WriteError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, errAlready):
		httpx.WriteError(w, http.StatusConflict, "already_following", "a follow or request already exists")
	case errors.Is(err, errNotFollowing):
		httpx.WriteError(w, http.StatusNotFound, "not_following", "you are not following this user")
	default:
		h.log.Error("follows", "err", err, "method", r.Method, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

// idParam reads a numeric path wildcard
func idParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}
