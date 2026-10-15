package notifications

import (
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

// GET /api/notifications?limit=&offset=
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	feed, err := h.svc.Feed(r.Context(), uid, limit, offset)
	if err != nil {
		h.log.Error("notifications", "err", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, feed)
}

// POST /api/notifications/{id}/read
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "notification id must be a number")
		return
	}
	if err := h.svc.MarkRead(r.Context(), uid, id); err != nil {
		h.log.Error("notifications", "err", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/notifications/read-all
func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	if err := h.svc.MarkAllRead(r.Context(), uid); err != nil {
		h.log.Error("notifications", "err", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
