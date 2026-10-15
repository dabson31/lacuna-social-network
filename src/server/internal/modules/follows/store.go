package follows

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"social/src/server/pkg/db/sqlite"
)

var (
	errSelf         = errors.New("cannot follow yourself")
	errUserMissing  = errors.New("user not found")
	errAlready      = errors.New("a follow or request already exists")
	errNoRequest    = errors.New("no such follow request")
	errNotFollowing = errors.New("not following")
)

// the store owns the follows table, it also reads users is_public because the follow
// action needs to know whether to auto accept, reading the column is not a module import
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// targetState returns whether the target exists and whether they are public, one check
// instead of two, a missing row means the user does not exist
func (s *Store) targetState(ctx context.Context, id int64) (isPublic bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT is_public FROM users WHERE id = ?`, id).Scan(&isPublic)
	if errors.Is(err, sql.ErrNoRows) {
		return false, errUserMissing
	}
	if err != nil {
		return false, fmt.Errorf("target state: %w", err)
	}
	return isPublic, nil
}

// insert creates the follow row at the given status, a unique violation means the
// pair already exists so we surface it as errAlready for the service to map to 409, which is a conflict
func (s *Store) insert(ctx context.Context, followerID, followeeID int64, status string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO follows (follower_id, followee_id, status) VALUES (?, ?, ?)`,
		followerID, followeeID, status,
	)
	if sqlite.IsUniqueViolation(err, "follows.follower_id") || sqlite.IsUniqueViolation(err, "follower_id") {
		return 0, errAlready
	}
	if err != nil {
		return 0, fmt.Errorf("insert follow: %w", err)
	}
	return res.LastInsertId()
}

// remove deletes the row in either direction of status, it doubles as cancel request
// and as unfollow, errNotFollowing means there was nothing there to delete
//
// ! it hands back the deleted row's id and status because the two cases are not the same
// ! event: withdrawing a PENDING request retracts a notification somebody is still being
// ! shown, while unfollowing an ACCEPTED follow retracts nothing — the request it came from
// ! was answered, and that answer is history the reader is entitled to keep
//
// ? RETURNING, so the status is read off the row that was actually deleted rather than by a
// ? SELECT before the DELETE. two unfollows racing would both pass that SELECT and both
// ? publish, and the second would be publishing about a row the first already removed. this
// ? is the same reasoning that keeps the author scope inside the post delete's own WHERE
func (s *Store) remove(ctx context.Context, followerID, followeeID int64) (id int64, status string, err error) {
	err = s.db.QueryRowContext(ctx,
		`DELETE FROM follows WHERE follower_id = ? AND followee_id = ? RETURNING id, status`,
		followerID, followeeID).Scan(&id, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", errNotFollowing
	}
	if err != nil {
		return 0, "", fmt.Errorf("remove follow: %w", err)
	}
	return id, status, nil
}

// requestParties returns who sent and who received a given follow row plus its status,
// the service uses the followee to check that only the receiver may accept or decline
func (s *Store) requestParties(ctx context.Context, followID int64) (followerID, followeeID int64, status string, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT follower_id, followee_id, status FROM follows WHERE id = ?`, followID,
	).Scan(&followerID, &followeeID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, "", errNoRequest
	}
	if err != nil {
		return 0, 0, "", fmt.Errorf("request parties: %w", err)
	}
	return followerID, followeeID, status, nil
}

func (s *Store) setStatus(ctx context.Context, followID int64, status string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE follows SET status = ? WHERE id = ?`, status, followID); err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	return nil
}

func (s *Store) deleteByID(ctx context.Context, followID int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM follows WHERE id = ?`, followID); err != nil {
		return fmt.Errorf("delete follow: %w", err)
	}
	return nil
}

// isAcceptedFollower answers can viewer see owner's follower lists, true when there is
// an accepted row from viewer to owner
// ! basically can i see someone elses
func (s *Store) isAcceptedFollower(ctx context.Context, followerID, followeeID int64) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM follows WHERE follower_id = ? AND followee_id = ? AND status = 'accepted'`,
		followerID, followeeID,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("is accepted follower: %w", err)
	}
	return true, nil
}

// incoming lists the pending requests aimed at this user, newest first, joined with
// the sender so the inbox can show a name and avatar without a second call
func (s *Store) incoming(ctx context.Context, followeeID int64) ([]RequestItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT f.id, u.id, u.first_name, u.last_name, u.avatar_path, f.created_at
		 FROM follows f
		 JOIN users u ON u.id = f.follower_id
		 WHERE f.followee_id = ? AND f.status = 'pending'
		 ORDER BY f.id DESC`, followeeID,
	)
	if err != nil {
		return nil, fmt.Errorf("incoming requests: %w", err)
	}
	defer rows.Close()

	var out []RequestItem
	for rows.Next() {
		var it RequestItem
		if err := rows.Scan(&it.FollowID, &it.ActorID, &it.FirstName, &it.LastName, &it.AvatarPath, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// listFollowers returns the accepted people who follow owner, listFollowing the people
// owner follows, both share a shape so one scanner serves both directions
func (s *Store) listFollowers(ctx context.Context, ownerID int64) ([]UserCard, error) {
	return s.cards(ctx,
		`SELECT u.id, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
		 FROM follows f JOIN users u ON u.id = f.follower_id
		 WHERE f.followee_id = ? AND f.status = 'accepted'
		 ORDER BY u.first_name, u.last_name`, ownerID)
}

func (s *Store) listFollowing(ctx context.Context, ownerID int64) ([]UserCard, error) {
	return s.cards(ctx,
		`SELECT u.id, u.first_name, u.last_name, u.nickname, u.avatar_path, u.is_public
		 FROM follows f JOIN users u ON u.id = f.followee_id
		 WHERE f.follower_id = ? AND f.status = 'accepted'
		 ORDER BY u.first_name, u.last_name`, ownerID)
}

func (s *Store) cards(ctx context.Context, query string, id int64) ([]UserCard, error) {
	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer rows.Close()

	var out []UserCard
	for rows.Next() {
		var c UserCard
		if err := rows.Scan(&c.ID, &c.FirstName, &c.LastName, &c.Nickname, &c.AvatarPath, &c.IsPublic); err != nil {
			return nil, fmt.Errorf("scan card: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// connected checks if there is an accepted follow between two people, going either way.
// either way matters because the dm rule is symmetric: you can message someone who
// follows you just as much as someone you follow, so one query has to cover both rows
// ? the args are a, b then b, a. same pair, both directions
// isAcceptedFollower above is the one-directional version, it answers "can i see your
// stuff", which is a different question and stays separate
func (s *Store) connected(ctx context.Context, a, b int64) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM follows
		  WHERE status = 'accepted'
		    AND ((follower_id = ? AND followee_id = ?) OR (follower_id = ? AND followee_id = ?))
		  LIMIT 1`, a, b, b, a).Scan(&one)

	// no row just means they arent connected, thats not a failure
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("connected: %w", err)
	}
	return true, nil
}
