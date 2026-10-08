package groups

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"social/src/server/pkg/db/sqlite"
)

var (
	errGroupMissing = errors.New("group not found")
	errMemberExists = errors.New("a membership or request already exists")
	errNoRow        = errors.New("no such row")
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// createGroup writes the group and the creator's own member row in one transaction, so
// a group can never exist without its creator as a member, the creator row carries the
// creator role which is what later lets only them approve join requests
func (s *Store) createGroup(ctx context.Context, creatorID int64, in CreateGroupInput) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO groups (creator_id, title, description, join_policy) VALUES (?, ?, ?, ?)`,
		creatorID, in.Title, in.Description, in.JoinPolicy,
	)
	if err != nil {
		return 0, fmt.Errorf("insert group: %w", err)
	}
	groupID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, status, role) VALUES (?, ?, 'member', 'creator')`,
		groupID, creatorID); err != nil {
		return 0, fmt.Errorf("insert creator member: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return groupID, nil
}

// the shared select for a group, it counts members and left joins the caller's own row
// so one query yields the group and how the caller stands with it, me can be null
const selectGroup = `
	SELECT g.id, g.creator_id, g.title, g.description, g.join_policy, g.created_at,
	       (SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id AND m.status = 'member') AS member_count,
	       COALESCE(me.status, ''), COALESCE(me.role, '')
	FROM groups g
	LEFT JOIN group_members me ON me.group_id = g.id AND me.user_id = ?`

func scanGroup(row interface{ Scan(...any) error }) (Group, error) {
	var g Group
	err := row.Scan(&g.ID, &g.CreatorID, &g.Title, &g.Description, &g.JoinPolicy, &g.CreatedAt,
		&g.MemberCount, &g.MyStatus, &g.MyRole)
	return g, err
}

func (s *Store) byID(ctx context.Context, viewerID, groupID int64) (Group, error) {
	g, err := scanGroup(s.db.QueryRowContext(ctx, selectGroup+` WHERE g.id = ?`, viewerID, groupID))
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, errGroupMissing
	}
	if err != nil {
		return Group{}, fmt.Errorf("group by id: %w", err)
	}
	return g, nil
}

// browse lists every group, this is how a user finds a group to ask to join, each row
// still carries the caller's own status so the ui knows which button to show
func (s *Store) browse(ctx context.Context, viewerID int64) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx, selectGroup+` ORDER BY g.id DESC`, viewerID)
	if err != nil {
		return nil, fmt.Errorf("browse groups: %w", err)
	}
	defer rows.Close()

	var out []Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// membership returns the caller's status and role in a group, both empty strings mean
// there is no row, so they are a stranger to the group, this one query backs the
// authorization check at the top of nearly every group action
func (s *Store) membership(ctx context.Context, groupID, userID int64) (status, role string, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT status, role FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID,
	).Scan(&status, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("membership: %w", err)
	}
	return status, role, nil
}

func (s *Store) members(ctx context.Context, groupID int64) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id, u.first_name, u.last_name, u.avatar_path, m.role
		 FROM group_members m JOIN users u ON u.id = m.user_id
		 WHERE m.group_id = ? AND m.status = 'member'
		 ORDER BY m.role ASC, u.first_name`, groupID)
	if err != nil {
		return nil, fmt.Errorf("members: %w", err)
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.FirstName, &m.LastName, &m.AvatarPath, &m.Role); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// addMember inserts a row at the given status, invitedBy is null for a self made join
// request, a unique violation means a row for this pair already exists
func (s *Store) addMember(ctx context.Context, groupID, userID int64, status string, invitedBy *int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, status, role, invited_by) VALUES (?, ?, ?, 'member', ?)`,
		groupID, userID, status, invitedBy,
	)
	if sqlite.IsUniqueViolation(err, "group_members.group_id") || sqlite.IsUniqueViolation(err, "group_id") {
		return errMemberExists
	}
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

// promote flips a pending invited or requested row to member, rowsAffected of zero means
// there was no matching pending row so the caller is told there is nothing to act on
func (s *Store) promote(ctx context.Context, groupID, userID int64, fromStatus string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE group_members SET status = 'member' WHERE group_id = ? AND user_id = ? AND status = ?`,
		groupID, userID, fromStatus)
	if err != nil {
		return fmt.Errorf("promote member: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errNoRow
	}
	return nil
}

// removeMember deletes a membership row in any state, it covers decline, leave and the
// creator rejecting a request, fromStatus narrows it when we only mean to remove a pending row
func (s *Store) removeMember(ctx context.Context, groupID, userID int64, fromStatus string) error {
	var res sql.Result
	var err error
	if fromStatus == "" {
		res, err = s.db.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID)
	} else {
		res, err = s.db.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ? AND user_id = ? AND status = ?`, groupID, userID, fromStatus)
	}
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errNoRow
	}
	return nil
}

// invitationsFor lists the groups this user has been invited to but has not answered,
// it joins the inviter so the invite can show who sent it
func (s *Store) invitationsFor(ctx context.Context, userID int64) ([]Invitation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.title, g.description, m.invited_by, inv.first_name || ' ' || inv.last_name
		 FROM group_members m
		 JOIN groups g ON g.id = m.group_id
		 LEFT JOIN users inv ON inv.id = m.invited_by
		 WHERE m.user_id = ? AND m.status = 'invited'
		 ORDER BY m.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("invitations: %w", err)
	}
	defer rows.Close()

	var out []Invitation
	for rows.Next() {
		var it Invitation
		if err := rows.Scan(&it.GroupID, &it.Title, &it.Description, &it.InvitedByID, &it.InvitedBy); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) requestsFor(ctx context.Context, groupID int64) ([]JoinRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id, u.first_name, u.last_name, u.avatar_path
		 FROM group_members m JOIN users u ON u.id = m.user_id
		 WHERE m.group_id = ? AND m.status = 'requested'
		 ORDER BY m.id DESC`, groupID)
	if err != nil {
		return nil, fmt.Errorf("requests: %w", err)
	}
	defer rows.Close()

	var out []JoinRequest
	for rows.Next() {
		var jr JoinRequest
		if err := rows.Scan(&jr.UserID, &jr.FirstName, &jr.LastName, &jr.AvatarPath); err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		out = append(out, jr)
	}
	return out, rows.Err()
}

// ---- group content ----

func (s *Store) addPost(ctx context.Context, groupID, authorID int64, content string, imagePath *string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO group_posts (group_id, author_id, content, image_path) VALUES (?, ?, ?, ?)`,
		groupID, authorID, content, imagePath)
	if err != nil {
		return 0, fmt.Errorf("add group post: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) posts(ctx context.Context, groupID int64) ([]GroupPost, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.id, p.group_id, p.author_id, p.content, p.image_path, p.created_at,
		        u.first_name || ' ' || u.last_name, u.avatar_path,
		        (SELECT COUNT(*) FROM group_comments c WHERE c.group_post_id = p.id)
		 FROM group_posts p JOIN users u ON u.id = p.author_id
		 WHERE p.group_id = ? ORDER BY p.id DESC`, groupID)
	if err != nil {
		return nil, fmt.Errorf("group posts: %w", err)
	}
	defer rows.Close()

	var out []GroupPost
	for rows.Next() {
		var p GroupPost
		if err := rows.Scan(&p.ID, &p.GroupID, &p.AuthorID, &p.Content, &p.ImagePath, &p.CreatedAt, &p.AuthorName, &p.AuthorAvatar, &p.CommentCount); err != nil {
			return nil, fmt.Errorf("scan group post: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// postGroup returns which group a group post belongs to, used to check the commenter is
// a member of that same group, zero means the post does not exist
func (s *Store) postGroup(ctx context.Context, postID int64) (int64, error) {
	var gid int64
	err := s.db.QueryRowContext(ctx, `SELECT group_id FROM group_posts WHERE id = ?`, postID).Scan(&gid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errNoRow
	}
	if err != nil {
		return 0, fmt.Errorf("post group: %w", err)
	}
	return gid, nil
}

func (s *Store) addComment(ctx context.Context, postID, authorID int64, content string, imagePath *string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO group_comments (group_post_id, author_id, content, image_path) VALUES (?, ?, ?, ?)`,
		postID, authorID, content, imagePath)
	if err != nil {
		return 0, fmt.Errorf("add group comment: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) comments(ctx context.Context, postID int64) ([]GroupComment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.group_post_id, c.author_id, c.content, c.image_path, c.created_at,
		        u.first_name || ' ' || u.last_name, u.avatar_path
		 FROM group_comments c JOIN users u ON u.id = c.author_id
		 WHERE c.group_post_id = ? ORDER BY c.id ASC`, postID)
	if err != nil {
		return nil, fmt.Errorf("group comments: %w", err)
	}
	defer rows.Close()

	var out []GroupComment
	for rows.Next() {
		var c GroupComment
		if err := rows.Scan(&c.ID, &c.GroupPostID, &c.AuthorID, &c.Content, &c.ImagePath, &c.CreatedAt, &c.AuthorName, &c.AuthorAvatar); err != nil {
			return nil, fmt.Errorf("scan group comment: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---- events ----

func (s *Store) addEvent(ctx context.Context, groupID, creatorID int64, in CreateEventInput, eventTime string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO events (group_id, creator_id, title, description, event_time) VALUES (?, ?, ?, ?, ?)`,
		groupID, creatorID, in.Title, in.Description, eventTime)
	if err != nil {
		return 0, fmt.Errorf("add event: %w", err)
	}
	return res.LastInsertId()
}

// events lists a group's events soonest first with the going and not going tallies and
// the caller's own answer, the correlated subqueries keep it to a single round trip
func (s *Store) events(ctx context.Context, groupID, viewerID int64) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.group_id, e.creator_id, e.title, e.description, e.event_time, e.created_at,
		        (SELECT COUNT(*) FROM event_responses r WHERE r.event_id = e.id AND r.response = 'going'),
		        (SELECT COUNT(*) FROM event_responses r WHERE r.event_id = e.id AND r.response = 'not_going'),
		        COALESCE((SELECT r.response FROM event_responses r WHERE r.event_id = e.id AND r.user_id = ?), '')
		 FROM events e WHERE e.group_id = ? ORDER BY e.event_time ASC`, viewerID, groupID)
	if err != nil {
		return nil, fmt.Errorf("events: %w", err)
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.GroupID, &e.CreatorID, &e.Title, &e.Description, &e.EventTime, &e.CreatedAt, &e.GoingCount, &e.NotGoingCount, &e.MyResponse); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) eventGroup(ctx context.Context, eventID int64) (int64, error) {
	var gid int64
	err := s.db.QueryRowContext(ctx, `SELECT group_id FROM events WHERE id = ?`, eventID).Scan(&gid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errNoRow
	}
	if err != nil {
		return 0, fmt.Errorf("event group: %w", err)
	}
	return gid, nil
}

// respond records the caller's answer, the composite primary key means a second answer
// from the same user updates the first instead of adding a row, so changing your mind
// can never leave two votes behind
func (s *Store) respond(ctx context.Context, eventID, userID int64, response string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO event_responses (event_id, user_id, response) VALUES (?, ?, ?)
		 ON CONFLICT (event_id, user_id) DO UPDATE SET response = excluded.response`,
		eventID, userID, response)
	if err != nil {
		return fmt.Errorf("respond: %w", err)
	}
	return nil
}

// memberIDs returns the user ids of the group's members, except one, used to fan an event
// out to everyone but its creator, membership is read at send time so it is always current
func (s *Store) memberIDs(ctx context.Context, groupID, except int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id FROM group_members WHERE group_id = ? AND status = 'member' AND user_id <> ?`,
		groupID, except)
	if err != nil {
		return nil, fmt.Errorf("member ids: %w", err)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan member id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// userExists is a cheap check so inviting a user who is not real reads as not found
// instead of surfacing as a foreign key error from the insert
func (s *Store) userExists(ctx context.Context, id int64) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("user exists: %w", err)
	}
	return true, nil
}
