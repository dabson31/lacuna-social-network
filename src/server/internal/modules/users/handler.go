package users

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

// GET /api/users/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	targetID, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "user id must be a number")
		return
	}
	p, err := h.svc.Get(r.Context(), viewerID, targetID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

// PATCH /api/users/me
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	var in UpdateProfileInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := h.svc.Update(r.Context(), viewerID, in); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PATCH /api/users/me/privacy
func (h *Handler) SetPrivacy(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	var in PrivacyInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := h.svc.SetPrivacy(r.Context(), viewerID, in.IsPublic); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/users?q=&limit=&offset=
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	list, err := h.svc.Search(r.Context(), viewerID, q, limit, offset)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	// gotta hand back an empty result in the form of an empty array, not a literal null
	if list == nil {
		list = []ListUser{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var fields httpx.ValidationErrors
	switch {
	case errors.As(err, &fields):
		httpx.WriteValidation(w, fields)
	case errors.Is(err, errNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "user not found")
	case errors.Is(err, errNicknameTaken):
		httpx.WriteErrorFields(w, http.StatusConflict, "nickname_taken", "this nickname is already taken",
			httpx.ValidationErrors{"nickname": "already taken"})
	case errors.Is(err, errNothingToDo):
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "no fields to update")
	default:
		h.log.Error("users", "err", err, "method", r.Method, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

// idParam reads a numeric path wildcard like the id in users id
func idParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}
