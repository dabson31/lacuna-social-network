package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"social/src/server/pkg/db/sqlite"
)

var (
	errNotFound      = errors.New("user not found")
	errEmailTaken    = errors.New("email already taken")
	errNicknameTaken = errors.New("nickname already taken")
)

// only sql, no rules nor http
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// userColumns is shared by every select, so scanUser can always match the query
const userColumns = `id, email, first_name, last_name, date_of_birth, avatar_path, nickname, about_me, is_public, created_at`

// both *sql.Row and *sql.Rows have this can method, so scanUser works for single rows and loops
type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.DateOfBirth, &u.AvatarPath, &u.Nickname, &u.AboutMe, &u.IsPublic, &u.CreatedAt)
	return u, err
}

func (s *Store) createUser(ctx context.Context, u User, passwordHash string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
	INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth, avatar_path, nickname, about_me)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Email, passwordHash, u.FirstName, u.LastName, u.DateOfBirth, u.AvatarPath, u.Nickname, u.AboutMe,
	)
	switch {
	case sqlite.IsUniqueViolation(err, "users.email"):
		return 0, errEmailTaken
	case sqlite.IsUniqueViolation(err, "users.nickname"):
		return 0, errNicknameTaken
	case err != nil:
		return 0, fmt.Errorf("insert user: %w", err)
	}
	return res.LastInsertId()

}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, errNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("user by id: %w", err)
	}
	return u, nil
}

// this func also returns the hash cause login is THE place that needs it
func (s *Store) UserByEmail(ctx context.Context, email string) (User, string, error) {
	var u User
	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+`, password_hash FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.DateOfBirth,
		&u.AvatarPath, &u.Nickname, &u.AboutMe, &u.IsPublic, &u.CreatedAt, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", errNotFound
	}
	if err != nil {
		return User{}, "", fmt.Errorf("user by email: %w", err)
	}
	return u, hash, nil
}
