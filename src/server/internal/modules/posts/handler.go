package posts

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/session"
	"social/src/server/internal/platform/storage"
)

// uploads above this size spill to a temp file instead of memory, the whole body cap
// is enforced earlier by the max body middleware
const multipartMemory = 1 << 20

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// POST /api/posts, multipart because a post can carry an image, viewer_ids is repeated
// once per chosen follower and only read for a private post
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	authorID, _ := session.UserID(r.Context())
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "expected a multipart/form-data body")
		return
	}
	defer r.MultipartForm.RemoveAll()

	in := CreatePostInput{
		Content: r.PostFormValue("content"),
		Privacy: r.PostFormValue("privacy"),
	}
	for _, raw := range r.PostForm["viewer_ids"] {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			in.ViewerIDs = append(in.ViewerIDs, id)
		}
	}
	if file, _, err := r.FormFile("image"); err == nil {
		defer file.Close()
		in.Image = file
	} else if !errors.Is(err, http.ErrMissingFile) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "could not read the image")
		return
	}

	post, err := h.svc.Create(r.Context(), authorID, in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, post)
}

// GET /api/posts?before=&limit=
func (h *Handler) Feed(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := h.svc.Feed(r.Context(), viewerID, before, limit)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	writeList(w, list)
}

// GET /api/users/{id}/posts, lives in the posts feature because it is about posts even
// though the url hangs off a user
func (h *Handler) ByAuthor(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	authorID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "user id must be a number")
		return
	}
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := h.svc.AuthorPosts(r.Context(), viewerID, authorID, before, limit)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	writeList(w, list)
}

// GET /api/posts/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "post id must be a number")
		return
	}
	post, err := h.svc.Get(r.Context(), viewerID, postID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, post)
}

// POST /api/posts/{id}/comments, multipart like a post because a comment can carry an image
func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "post id must be a number")
		return
	}
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "expected a multipart/form-data body")
		return
	}
	defer r.MultipartForm.RemoveAll()

	in := CreateCommentInput{Content: r.PostFormValue("content")}
	if file, _, err := r.FormFile("image"); err == nil {
		defer file.Close()
		in.Image = file
	} else if !errors.Is(err, http.ErrMissingFile) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "could not read the image")
		return
	}

	c, err := h.svc.Comment(r.Context(), viewerID, postID, in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, c)
}

// GET /api/posts/{id}/comments
func (h *Handler) Comments(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := session.UserID(r.Context())
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "post id must be a number")
		return
	}
	list, err := h.svc.Comments(r.Context(), viewerID, postID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if list == nil {
		list = []Comment{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func writeList(w http.ResponseWriter, list []Post) {
	if list == nil {
		list = []Post{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var fields httpx.ValidationErrors
	switch {
	case errors.As(err, &fields):
		httpx.WriteValidation(w, fields)
	case errors.Is(err, errNotVisible):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "post not found")
	case errors.Is(err, errForbiddenViewer):
		httpx.WriteError(w, http.StatusForbidden, "forbidden_viewer", "a chosen viewer is not an accepted follower")
	case errors.Is(err, storage.ErrUnsupportedType):
		httpx.WriteErrorFields(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error(),
			httpx.ValidationErrors{"image": "must be a JPEG, PNG or GIF"})
	default:
		h.log.Error("posts", "err", err, "method", r.Method, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

// DELETE /api/posts/{id}, the design's "withdraw" action. only the author may do it, and
// anybody else gets the same 404 an invisible post gives
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	authorID, _ := session.UserID(r.Context())
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "post id must be a number")
		return
	}
	if err := h.svc.Delete(r.Context(), authorID, postID); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
