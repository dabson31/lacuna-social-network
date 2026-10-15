package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/session"
	"social/src/server/internal/platform/storage"
)

// if the file is any bigger than this, it goes to a temporary file instead of being held in memory
// (whole body hard cap is in app.go)
const multipartMemory = 1 << 20

// only translates http <-> service (decode request, call service and write response)
type Handler struct {
	svc      *Service
	sessions *session.Store
	log      *slog.Logger
}

// said process
func NewHandler(svc *Service, sessions *session.Store, log *slog.Logger) *Handler {
	return &Handler{svc: svc, sessions: sessions, log: log}
}

// POST /api/auth/register (multipart/form-data cause of optional avatar file)
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "too_large", "request body is too large")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "expected a multipart/form-data body")
		return
	}
	// the file that we stored in temp, yeah its gone, deleted here in case a middleware swapped the req
	// (net/http does this for us, except if said middleware swaps the request)
	defer r.MultipartForm.RemoveAll()

	in := RegisterInput{
		Email:       r.PostFormValue("email"),
		Password:    r.PostFormValue("password"),
		FirstName:   r.PostFormValue("first_name"),
		LastName:    r.PostFormValue("last_name"),
		DateOfBirth: r.PostFormValue("date_of_birth"),
		Nickname:    r.PostFormValue("nickname"),
		AboutMe:     r.PostFormValue("about_me"),
	}

	file, _, err := r.FormFile("avatar")
	switch {
	case err == nil:
		defer file.Close()
		in.Avatar = file
	case errors.Is(err, http.ErrMissingFile):
	default:
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "could not read the avatar file")
		return
	}

	user, sess, err := h.svc.Register(r.Context(), in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	h.sessions.SetCookie(w, sess)
	httpx.WriteJSON(w, http.StatusCreated, user)
}

// POST /api/auth/login === JSON: {"email": "EMAIL", "password": "PASSWORD"}
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in LoginInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	user, sess, err := h.svc.Login(r.Context(), in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	h.sessions.SetCookie(w, sess)
	httpx.WriteJSON(w, http.StatusOK, user)
}

// POST /api/auth/logout (requireAuth)
// deletes the row AND clears the cookies obviously
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// RequireAuth already looked this session up and put it in the context, so the cookie
	// does not need re-reading. the id is what identifies the login being ended, and the user
	// id is how the hub finds the sockets it opened without walking every socket on the server
	sess, _ := session.Current(r.Context())
	if err := h.svc.Logout(r.Context(), sess.UserID, sess.ID); err != nil {
		h.writeErr(w, r, err)
		return
	}
	h.sessions.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/auth/me (RequireAuth) called in the frontend to see who's logged in (who's me)
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, _ := session.UserID(r.Context())
	user, err := h.svc.Me(r.Context(), userID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}

// writeErr is the one place that maps service errors to status codes
func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var fields httpx.ValidationErrors
	switch {
	case errors.As(err, &fields):
		httpx.WriteValidation(w, fields)
	case errors.Is(err, errEmailTaken):
		httpx.WriteErrorFields(w, http.StatusConflict, "email_taken", "this email is already registered",
			httpx.ValidationErrors{"email": "already registered"})
	case errors.Is(err, errNicknameTaken):
		httpx.WriteErrorFields(w, http.StatusConflict, "nickname_taken", "this nickname is already taken",
			httpx.ValidationErrors{"nickname": "already taken"})
	case errors.Is(err, ErrInvalidCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	case errors.Is(err, storage.ErrUnsupportedType):
		httpx.WriteErrorFields(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error(),
			httpx.ValidationErrors{"avatar": "must be a JPEG, PNG or GIF"})
	case errors.Is(err, errNotFound):
		// a valid session for a user that no longer exists
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "you must be logged in")
	default:
		h.log.Error("auth", "err", err, "method", r.Method, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}
