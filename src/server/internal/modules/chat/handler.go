package chat

import (
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

// POST /api/messages
func (h *Handler) SendDM(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	var in SendDMInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	msg, err := h.svc.SendDM(r.Context(), uid, in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	// 201 with the saved row, so the client can swap its optimistic bubble for the real id
	// and the server's timestamp. the socket frame it also receives is then a duplicate it
	// can throw away by id
	httpx.WriteJSON(w, http.StatusCreated, msg)
}

// GET /api/messages/{userID}?limit=&before=
func (h *Handler) DMHistory(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	peer, err := idParam(r, "userID")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "user id must be a number")
		return
	}
	limit, before := pageParams(r)
	list, err := h.svc.DMHistory(r.Context(), uid, peer, limit, before)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []DirectMessage{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// POST /api/messages/{userID}/read
func (h *Handler) MarkDMRead(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	peer, err := idParam(r, "userID")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "user id must be a number")
		return
	}
	if err := h.svc.MarkDMRead(r.Context(), uid, peer); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/conversations
func (h *Handler) Inbox(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	inbox, err := h.svc.Inbox(r.Context(), uid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, inbox)
}

// POST /api/groups/{id}/messages
func (h *Handler) SendGroupMessage(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	var in SendGroupMessageInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	msg, err := h.svc.SendGroupMessage(r.Context(), gid, uid, in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, msg)
}

// GET /api/groups/{id}/messages?limit=&before=
func (h *Handler) GroupHistory(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	limit, before := pageParams(r)
	list, err := h.svc.GroupHistory(r.Context(), gid, uid, limit, before)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []GroupMessage{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// POST /api/groups/{id}/messages/read
func (h *Handler) MarkGroupRead(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	if err := h.svc.MarkGroupRead(r.Context(), gid, uid); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pageParams reads the cursor pair every history endpoint takes
//! before is a message id and not an offset, because messages keep coming in while youre
//! scrolling. with an offset the page boundary slides and you get shown the same message
//! twice. zero means start from the newest
func pageParams(r *http.Request) (limit int, before int64) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	before, _ = strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	return limit, before
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var fields httpx.ValidationErrors
	switch {
	case errors.As(err, &fields):
		httpx.WriteValidation(w, fields)
	case errors.Is(err, errForbidden):
		//! 403 and not 404 here. the recipient existing isnt a secret, theyre findable in
		//! search anyway, its only the permission to write to them thats missing. this is
		//! the status the audit's "two unconnected users cant chat" check lands on
		httpx.WriteError(w, http.StatusForbidden, "chat_not_allowed",
			"you can only message people you follow, who follow you, or whose profile is public")
	case errors.Is(err, errNotFound), errors.Is(err, errNoRow):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "not found")
	default:
		h.log.Error("chat", "err", err, "method", r.Method, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

func idParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}
