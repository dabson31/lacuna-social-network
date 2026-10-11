package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"social/src/server/pkg/db/sqlite"
)

var (
	errNotFound      = errors.New("user not found")
	errNicknameTaken = errors.New("nickname already taken")
	errNothingToDo   = errors.New("no fields to update")
)

// store is only sql
// it reads the follows and users table cause the follower count and viewer relationship live there
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) base(ctx context.Context, id int64) (raw, error) {
	var r raw
	err := s.db.QueryRowContext(ctx,
		`SELECT id, first_name, last_name, avatar_path, nickname, about_me, email, date_of_birth, is_public, created_at
		 FROM users WHERE id = ?`, id,
	).Scan(&r.ID, &r.FirstName, &r.LastName, &r.AvatarPath, &r.Nickname, &r.AboutMe, &r.Email, &r.DateOfBirth, &r.IsPublic, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return raw{}, errNotFound
	}
	if err != nil {
		return raw{}, fmt.Errorf("user base: %w", err)
	}
	return r, nil
}

// returns accepted followers/following, pending does not count for any of the two. since both
// are displayed on every profile we just fetch them together
func (s *Store) counts(ctx context.Context, id int64) (followers, following int, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT
		   (SELECT COUNT(*) FROM follows WHERE followee_id = ? AND status = 'accepted'),
		   (SELECT COUNT(*) FROM follows WHERE follower_id = ? AND status = 'accepted')`,
		id, id,
	).Scan(&followers, &following)
	if err != nil {
		return 0, 0, fmt.Errorf("user counts: %w", err)
	}
	return followers, following, nil
}

// this is status of the row where the viewer follows a specific target, if its empty it means
// that there is no row at all so there are no requests and no follows
func (s *Store) relationship(ctx context.Context, viewerID, targetID int64) (string, error) {
	var status string
	err := s.db.QueryRowContext(ctx,
		`SELECT status FROM follows WHERE follower_id = ? AND followee_id = ?`, viewerID, targetID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("relationship: %w", err)
	}
	return status, nil
}

// creates the SET only from the fields that were sent
func (s *Store) update(ctx context.Context, id int64, in UpdateProfileInput) error {
	var sets []string
	var args []any

	if in.FirstName != nil {
		sets = append(sets, "first_name = ?")
		args = append(args, strings.TrimSpace(*in.FirstName))
	}
	if in.LastName != nil {
		sets = append(sets, "last_name = ?")
		args = append(args, strings.TrimSpace(*in.LastName))
	}
	if in.Nickname != nil {
		sets = append(sets, "nickname = ?")
		args = append(args, nullIfEmpty(*in.Nickname))
	}
	if in.AboutMe != nil {
		sets = append(sets, "about_me = ?")
		args = append(args, nullIfEmpty(*in.AboutMe))
	}
	if len(sets) == 0 {
		return errNothingToDo
	}

	args = append(args, id)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if sqlite.IsUniqueViolation(err, "users.nickname") {
		return errNicknameTaken
	}
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}

func (s *Store) setPrivacy(ctx context.Context, id int64, isPublic bool) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET is_public = ? WHERE id = ?`, isPublic, id); err != nil {
		return fmt.Errorf("set privacy: %w", err)
	}
	return nil
}

// this matches the query against name and nickname where we leave the caller out,
// ! email is deliberately NOT matched. it is not in ListUser either, but a LIKE on it leaks
// ! the column anyway: a caller who guesses "@gmail.com" or a full address gets a hit, and a
// ! hit is a yes/no answer about who holds that address. the same answer auth refuses to give
// ! on purpose, where login returns one error for a wrong email and a wrong password and
// ! burns the same 50ms either way (auth/service.go) so the response never confirms which
// ! addresses are registered. matching on it here undid that in a search box
// ? a user who wants to be findable by something short has a nickname for it, which is public
// ? by design and is what the directory prints as a handle
func (s *Store) search(ctx context.Context, viewerID int64, q string, limit, offset int) ([]ListUser, error) {
	like := "%" + q + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, first_name, last_name, nickname, avatar_path, is_public
		 FROM users
		 WHERE id <> ?
		   AND (first_name LIKE ? OR last_name LIKE ? OR nickname LIKE ?)
		 ORDER BY first_name, last_name
		 LIMIT ? OFFSET ?`,
		viewerID, like, like, like, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	var out []ListUser
	for rows.Next() {
		var u ListUser
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.AvatarPath, &u.IsPublic); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// turns blank strings into a real sql NULL, important for nickname because
// that column has unique constraints and empty strings are identical with sqlite
// unlike null which are handled as distinct values
func nullIfEmpty(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}

// state answers both halves of a visibility question at once: does this user exist at all,
// and is their profile public. one row instead of two separate queries
// base() already returns all of this, but it reads 10 columns and fills a raw struct, which
// is a lot of work for a yes/no check that runs every time someone sends a message
func (s *Store) state(ctx context.Context, id int64) (exists, isPublic bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT is_public FROM users WHERE id = ?`, id).Scan(&isPublic)

	// no row = no user. false, false is already the right answer so theres nothing to build
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("user state: %w", err)
	}
	return true, isPublic, nil
}
