package groups

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/session"
	"social/src/server/internal/platform/storage"
)

const multipartMemory = 1 << 20

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// POST /api/groups
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	var in CreateGroupInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	g, err := h.svc.Create(r.Context(), uid, in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, g)
}

// GET /api/groups
func (h *Handler) Browse(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	list, err := h.svc.Browse(r.Context(), uid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []Group{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// GET /api/groups/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	g, err := h.svc.Get(r.Context(), uid, gid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	// the member list is only filled for members, so for everyone else it is a nil slice —
	// and a nil slice marshals to null, not []
	// ! every other list route in this api does the same conversion, and this was the one that
	// ! did not: GroupDetail.members is typed Member[] on the other side, so a null there is
	// ! the contract lying. the page only reads it behind an isMember check today, which is
	// ! the single reason it has never crashed on .map
	if g.Members == nil {
		g.Members = []Member{}
	}
	httpx.WriteJSON(w, http.StatusOK, g)
}

// POST /api/groups/{id}/invite
func (h *Handler) Invite(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	var in InviteInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := h.svc.Invite(r.Context(), gid, uid, in.UserID); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/groups/{id}/invitations/accept
func (h *Handler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	h.simpleGroupAction(w, r, h.svc.AcceptInvite)
}

// POST /api/groups/{id}/invitations/decline
func (h *Handler) DeclineInvite(w http.ResponseWriter, r *http.Request) {
	h.simpleGroupAction(w, r, h.svc.DeclineInvite)
}

// DELETE /api/groups/{id}/membership
func (h *Handler) Leave(w http.ResponseWriter, r *http.Request) {
	h.simpleGroupAction(w, r, h.svc.Leave)
}

// simpleGroupAction is the shared body for the actions that take only the group id and
// the caller, fn is whichever transition applies, accept invite, decline invite and leave
func (h *Handler) simpleGroupAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64) error) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	if err := fn(r.Context(), gid, uid); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/invitations
func (h *Handler) Invitations(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	list, err := h.svc.Invitations(r.Context(), uid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []Invitation{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// POST /api/groups/{id}/join
func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	status, err := h.svc.RequestJoin(r.Context(), gid, uid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": status})
}

// GET /api/groups/{id}/requests
func (h *Handler) Requests(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	list, err := h.svc.Requests(r.Context(), gid, uid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []JoinRequest{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// POST /api/groups/{id}/requests/{userID}/accept
func (h *Handler) AcceptRequest(w http.ResponseWriter, r *http.Request) {
	h.decideRequest(w, r, true)
}

// POST /api/groups/{id}/requests/{userID}/decline
func (h *Handler) DeclineRequest(w http.ResponseWriter, r *http.Request) {
	h.decideRequest(w, r, false)
}

func (h *Handler) decideRequest(w http.ResponseWriter, r *http.Request, accept bool) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	target, err := idParam(r, "userID")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "user id must be a number")
		return
	}
	if accept {
		err = h.svc.AcceptRequest(r.Context(), gid, uid, target)
	} else {
		err = h.svc.DeclineRequest(r.Context(), gid, uid, target)
	}
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/groups/{id}/posts
func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	in, err := postInput(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	defer r.MultipartForm.RemoveAll()
	p, err := h.svc.CreatePost(r.Context(), gid, uid, in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, p)
}

// POST /api/groups/{id}/posts/read
// ? the mirror of chat's mark-read, and it answers 204 for the same reason: there is nothing
// ? to hand back, the next read of the group carries the cleared count
func (h *Handler) MarkPostsRead(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	if err := h.svc.MarkPostsRead(r.Context(), gid, uid); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/groups/{id}/posts
func (h *Handler) Posts(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	list, err := h.svc.Posts(r.Context(), gid, uid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []GroupPost{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// POST /api/groups/{id}/posts/{postID}/comments
func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	postID, err := idParam(r, "postID")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "post id must be a number")
		return
	}
	in, err := postInput(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	defer r.MultipartForm.RemoveAll()
	c, err := h.svc.CreateComment(r.Context(), postID, uid, in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, c)
}

// GET /api/groups/{id}/posts/{postID}/comments
func (h *Handler) Comments(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	postID, err := idParam(r, "postID")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "post id must be a number")
		return
	}
	list, err := h.svc.Comments(r.Context(), postID, uid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []GroupComment{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// POST /api/groups/{id}/events
func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	var in CreateEventInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	e, err := h.svc.CreateEvent(r.Context(), gid, uid, in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, e)
}

// GET /api/groups/{id}/events
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	gid, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "group id must be a number")
		return
	}
	list, err := h.svc.Events(r.Context(), gid, uid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []Event{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// POST /api/events/{id}/respond
func (h *Handler) Respond(w http.ResponseWriter, r *http.Request) {
	uid, _ := session.UserID(r.Context())
	eventID, err := idParam(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "event id must be a number")
		return
	}
	var in RespondInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := h.svc.Respond(r.Context(), eventID, uid, in); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// postInput parses the shared multipart body used by group posts and group comments
func postInput(r *http.Request) (GroupPostInput, error) {
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		return GroupPostInput{}, errors.New("expected a multipart/form-data body")
	}
	in := GroupPostInput{Content: r.PostFormValue("content")}
	if file, _, err := r.FormFile("image"); err == nil {
		in.Image = file
	} else if !errors.Is(err, http.ErrMissingFile) {
		return GroupPostInput{}, errors.New("could not read the image")
	}
	return in, nil
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var fields httpx.ValidationErrors
	switch {
	case errors.As(err, &fields):
		httpx.WriteValidation(w, fields)
	case errors.Is(err, errMemberExists):
		httpx.WriteError(w, http.StatusConflict, "already_exists", "a membership or request already exists")
	case errors.Is(err, errForbidden):
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you are not allowed to do that")
	case errors.Is(err, errGroupMissing), errors.Is(err, errNotFound), errors.Is(err, errNoRow):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, storage.ErrUnsupportedType):
		httpx.WriteErrorFields(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error(),
			httpx.ValidationErrors{"image": "must be a JPEG, PNG or GIF"})
	default:
		h.log.Error("groups", "err", err, "method", r.Method, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

func idParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}
