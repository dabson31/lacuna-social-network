package session

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// the cookie carrying the session id
const CookieName = "sid"

// if no such session and session expired; both 401
var ErrNotFound = errors.New("session not found or expired")

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
}

// store keeps sessions in the sessions table, meaning they can persist a server restart
// todo: check for browesers here
type Store struct {
	db     *sql.DB
	ttl    time.Duration
	secure bool // only true in prod because localhost is just http
}

func NewStore(db *sql.DB, ttl time.Duration, secure bool) *Store {
	return &Store{db: db, ttl: ttl, secure: secure}
}

func (s *Store) Create(ctx context.Context, userID int64) (Session, error) {
	// the driver stores times as text
	now := time.Now().UTC()

	sess := Session{
		// rand.Text is 26 characters of crypto-random base32, which is 128 bits, which is more than a uuid v4, which is 122 bits
		ID:        rand.Text(),
		UserID:    userID,
		ExpiresAt: now.Add(s.ttl),
	}

	// this cleans up expired sessions from the database
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now); err != nil {
		return Session{}, fmt.Errorf("sweep sessions: %w", err)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		sess.ID, sess.UserID, sess.ExpiresAt,
	)
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	return sess, nil
}

// expiry is checked in go, not sql, so we never depend on how the driver formats times as text
func (s *Store) Get(ctx context.Context, id string) (Session, error) {
	sess := Session{ID: id}
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM sessions WHERE id = ?`, id,
	).Scan(&sess.UserID, &sess.ExpiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}

	if time.Now().After(sess.ExpiresAt) {
		_ = s.Destroy(ctx, id)
		return Session{}, ErrNotFound
	}
	return sess, nil
}

func (s *Store) Destroy(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// the biscuit
func (s *Store) SetCookie(w http.ResponseWriter, sess Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.ID,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		MaxAge:   int(time.Until(sess.ExpiresAt).Seconds()),
		HttpOnly: true, // since js cant read (cant relate), an XSS bug cant steal the session
		Secure:   s.secure,
		// here, SameSite is based on the concept of a site, the fact we're using different ports dont matter
		SameSite: http.SameSiteLaxMode,
	})
}

// this function tells the browser to delete the cookie right away
func (s *Store) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// the context helpers live here (and not in middleware) so modules can read the user id
// without importing app/, which would be an import cycle: app imports the modules
// an unexported struct type as the key means no other package can collide with it
type ctxKey struct{}

// WithSession carries the whole session and not just whose it is.
// ! the id matters as much as the user does, and the websocket is why. a socket is
// ! authenticated once, during the handshake, and then lives for hours: when the session
// ! behind it is destroyed, something has to be able to find THAT socket. not every socket
// ! belonging to that user — the same person may be logged in from another browser on a
// ! session of its own, and those two have to stay independent, which the audit checks
// ? ExpiresAt rides along for the same reason, so a long lived connection can outlive nothing
func WithSession(ctx context.Context, sess Session) context.Context {
	return context.WithValue(ctx, ctxKey{}, sess)
}

// Current returns the session RequireAuth put in the context
// ok is false if RequireAuth didnt run on this route
func Current(ctx context.Context) (Session, bool) {
	sess, ok := ctx.Value(ctxKey{}).(Session)
	return sess, ok
}

// UserID returns the logged-in user's id
// ok is false if RequireAuth didnt run on this route
// ? kept as its own helper although Current now holds the same field: it is what forty odd
// ? handlers ask for, and none of them has any business with the rest of the session
func UserID(ctx context.Context) (int64, bool) {
	sess, ok := Current(ctx)
	return sess.UserID, ok
}
