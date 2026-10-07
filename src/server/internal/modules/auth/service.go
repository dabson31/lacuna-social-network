package auth

import (
	"context"
	"errors"
	"net/mail"
	"social-network/internal/platform/httpx"
	"social-network/internal/platform/password"
	"strings"
	"time"
	"unicode/utf8"

	"social/src/server/internal/platform/session"
	"social/src/server/internal/platform/storage"
)

// we keep the same error (ErrInvalidCredentials) for both invalid email and password options so users cannot find out which email is registered
var ErrInvalidCredentials = errors.New("invalid email or password")

// so this is a bcrypt hash of a random string. login compares with it when the email doesnt exist. both failure paths take the same 50ms ish and
// the response time doesnt leak which email exists
var dummyHash, _ = password.HashPassword("alep alep aleppppp")

type Service struct {
	store   *Store
	session *session.Store
	uploads *storage.Store
}

func NewService(store *Store, sessions *session.Store, uploads *storage.Store) *Service {
	return &Service{store: store, sessions: sessions, uploads: uploads}
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (User, session.Session, error) {
	in.Email = strings.TrimSpace(in.Email)
	in.FirstName = strings.TrimSpace(in.FirstName)
	in.LastName = strings.TrimSpace(in.LastName)
	in.DateOfBirth = strings.TrimSpace(in.DateOfBirth)

	if fields := validateRegister(in); fields.Any() {
		return User{}, session.Session{}, fields
	}

	hash, err := password.HashPassword(in.Password)
	if err != nil {
		return User{}, session.Session{}, err
	}

	u := User{
		Email:       in.Email,
		FirstName:   in.FirstName,
		LastName:    in.LastName,
		DateOfBirth: in.DateOfBirth,
		Nickname:    optional(in.Nickname),
		AboutMe:     optional(in.AboutMe),
	}

	// avatar is lastly saved, right after everything that can cheaply be rejected have passed
	// !less computation power needed wow very optimized! ^^^
	if in.Avatar != nil {
		name, err := s.uploads.Save(in.Avatar)
		if err != nil {
			return User{}, session.Session{}, err
		}
		u.AvatarPath = &name
	}

	id, err := s.store.CreateUser(ctx, u, hash)
	if err != nil {
		// the user row was never created, so the file we just saved would be orphaned
		if u.AvatarPath != nil {
			_ = s.uploads.Delete(*u.AvatarPath)
		}
		return User{}, session.Session{}, err
	}

	return s.startSession(ctx, id)
}

func (s *Service) Login(ctx context.Context, in LoginInput) (User, session.Session, error) {
	u, hash, err := s.store.UserByEmail(ctx, strings.TrimSpace(in.Email))
	if errors.Is(err, errNotFound) {
		_, _ = password.CheckPassword(dummyHash, in.Password)
		return User{}, session.Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, session.Session{}, err
	}

	ok, err := password.CheckPassword(hash, in.Password)
	if err != nil {
		return User{}, session.Session{}, err
	}
	if !ok {
		return User{}, session.Session{}, ErrInvalidCredentials
	}

	sess, err := s.sessions.Create(ctx, u.ID)
	if err != nil {
		return User{}, session.Session{}, err
	}
	return u, sess, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.sessions.Destroy(ctx, sessionID)
}

func (s *Service) Me(ctx context.Context, userID int64) (User, error) {
	return s.store.UserByID(ctx, userID)
}

// this re-reads the new user (created_at comes from the db) and logs them in
func (s *Service) startSession(ctx context.Context, userID int64) (User, session.Session, error) {
	u, err := s.store.UserByID(ctx, userID)
	if err != nil {
		return User{}, session.Session{}, err
	}
	sess, err := s.sessions.Create(ctx, userID)
	if err != nil {
		return User{}, session.Session{}, err
	}
	return u, sess, nil
}

func validateRegister(in RegisterInput) httpx.ValidationErrors {
	fields := httpx.ValidationErrors{}

	if in.Email == "" {
		fields.Add("email", "email is required")
	} else if addr, err := mail.ParseAddress(in.Email); err != nil || addr.Address != in.Email {
		// has to be a valid password, reject stuff like <hello@socialNetwork.com>
		fields.Add("email", "email is not valid")
	}

	switch {
	case in.Password == "":
		fields.Add("password", "password is required")
	case len(in.Password) < 8:
		fields.Add("password", "password must be at least 8 characters")
	case len(in.Password) > 72:
		//! this is very neat, handling bytes was something i was thinking about
		fields.Add("password", "password must be at most 72 bytes")
	}

	requiredText(fields, "first_name", in.FirstName, 50)
	requiredText(fields, "last_name", in.LastName, 50)

	if in.DateOfBirth == "" {
		fields.Add("date_of_birth", "date of birth is required")
	} else if dob, err := time.Parse(time.DateOnly, in.DateOfBirth); err != nil {
		fields.Add("date_of_birth", "date of birth must look like YYYY-MM-DD")
	} else if !dob.Before(time.Now()) {
		fields.Add("date_of_birth", "date of birth must be in the past")
	}

	if utf8.RuneCountInString(strings.TrimSpace(in.Nickname)) > 30 {
		fields.Add("nickname", "nickname must be at most 30 characters")
	}
	if utf8.RuneCountInString(in.AboutMe) > 500 {
		fields.Add("about_me", "about me must be at most 500 characters")
	}
	return fields
}

func requiredText(fields httpx.ValidationErrors, name, value string, max int) {
	if value == "" {
		fields.Add(name, strings.ReplaceAll(name, "_", " ")+" is required")
	} else if utf8.RuneCountInString(value) > max {
		fields.Add(name, strings.ReplaceAll(name, "_", " ")+" is too long")
	}
}

// turns empty form fields into null, important for nickname as its unique and sqlite treats every null
// as dinstinct, yet every "" is equal in its eyes meaning only one user would be allowed not to have
// a nickname (rare account guys 5000$)
func optional(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
