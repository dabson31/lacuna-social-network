package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// insert writes one notification and gives back its id, entityID may be null for a kind
// that points at nothing, the caller has already decided the recipient is not the actor
// ! it returns the id now because the service needs to read the row straight back out to
// ! push it, a notification frame with no actor name on it is useless to the frontend
func (s *Store) insert(ctx context.Context, userID int64, typ string, actorID int64, entityID *int64) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO notifications (user_id, type, actor_id, entity_id) VALUES (?, ?, ?, ?)`,
		userID, typ, actorID, entityID)
	if err != nil {
		return 0, fmt.Errorf("insert notification: %w", err)
	}
	return res.LastInsertId()
}

// groupOf resolves which group a notification belongs to, in sql, because entity_id names
// a different table per kind: the group itself for the two group kinds, an event for
// event_created, and a follows row for follow_request which belongs to no group at all
// ! reading the events table from this module is not a module import, it is one package
// ! reading one column of a shared schema, the same way follows/store.go reads
// ! users.is_public. going through the groups service instead would create exactly the
// ! cross module dependency the event bus exists to avoid
const groupOf = `
		CASE n.type
		     WHEN 'group_invite'       THEN n.entity_id
		     WHEN 'group_join_request' THEN n.entity_id
		     WHEN 'event_created'      THEN (SELECT e.group_id FROM events e WHERE e.id = n.entity_id)
		END`

// live hides an unread follow_request whose follows row is no longer pending.
// ! a follow_request is a POINTER at a follows row, and that row is deleted the moment the
// ! sender cancels the request. the notification outlives it, still unread, still offering an
// ! accept that now answers 404 — so requesting, cancelling and requesting again used to leave
// ! one dead row per attempt. this is the half of that fix that retires the dead ones
// ? only unread rows are filtered. a read one has been dealt with and stays in the channel as
// ? history, which is what the design's own empty copy promises: "resolved items stay in the
// ? notices channel"
// ? and only follow_request needs it. the other three kinds point at a group or an event,
// ? neither of which can disappear behind the notification's back — there is no cancel-an-invite
// ? route and no delete-a-group one
const live = `
	AND (n.type <> 'follow_request'
	     OR n.read_at IS NOT NULL
	     OR EXISTS (SELECT 1 FROM follows f WHERE f.id = n.entity_id AND f.status = 'pending'))`

// the one select both reads use, so a pushed frame and a row fetched from the inbox are
// identical by construction and the frontend needs one renderer, not two
const selectNotification = `
	SELECT n.id, n.type, n.actor_id, u.first_name || ' ' || u.last_name, u.avatar_path,
	       n.entity_id, ` + groupOf + `, n.read_at, n.created_at
	FROM notifications n JOIN users u ON u.id = n.actor_id`

func scanNotification(row interface{ Scan(...any) error }) (Notification, error) {
	var n Notification
	err := row.Scan(&n.ID, &n.Type, &n.ActorID, &n.ActorName, &n.ActorAvatar,
		&n.EntityID, &n.GroupID, &n.ReadAt, &n.CreatedAt)
	return n, err
}

// byID reads one row in the exact same shape list returns
func (s *Store) byID(ctx context.Context, id int64) (Notification, error) {
	n, err := scanNotification(s.db.QueryRowContext(ctx, selectNotification+` WHERE n.id = ?`, id))
	if err != nil {
		return Notification{}, fmt.Errorf("notification by id: %w", err)
	}
	return n, nil
}

// list returns the inbox newest first, joined with the actor for a name and avatar, the
// idx_notifications_inbox index on user_id and id descending backs this read
func (s *Store) list(ctx context.Context, userID int64, limit, offset int) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx,
		selectNotification+`
		 WHERE n.user_id = ?`+live+`
		 ORDER BY n.id DESC
		 LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// unreadCount counts the rows with no read_at, the partial idx_notifications_unread index
// holds only those rows so this stays cheap however much history piles up
func (s *Store) unreadCount(ctx context.Context, userID int64) (int, error) {
	var n int
	//! the same `live` predicate the list uses, and for the hardest reason to debug: a badge
	//! counting rows the list refuses to draw is a number nobody can clear
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications n WHERE n.user_id = ? AND n.read_at IS NULL`+live, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("unread count: %w", err)
	}
	return n, nil
}

// findUnread returns the id of the unread notification a new one would duplicate, or 0 when
// there is none. matchEntity is the caller's decision and the comment on Record explains it
// ? `entity_id IS ?` and not `=`, because a null entity has to match a null entity and `=`
// ? answers null to that, which is neither true nor a row
func (s *Store) findUnread(ctx context.Context, userID int64, typ string, actorID int64, entityID *int64, matchEntity bool) (int64, error) {
	q := `SELECT id FROM notifications
	       WHERE user_id = ? AND type = ? AND actor_id = ? AND read_at IS NULL`
	args := []any{userID, typ, actorID}
	if matchEntity {
		q += ` AND entity_id IS ?`
		args = append(args, entityID)
	}
	q += ` ORDER BY id DESC LIMIT 1`

	var id int64
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("find unread notification: %w", err)
	}
	return id, nil
}

// refresh points an existing notification at a new entity and restamps it.
// ! the entity is the point: a resent follow request is a NEW follows row, and a notification
// ! left pointing at the old one is the dead end `live` exists to hide
// ? created_at moves so the row reports when the request actually arrived rather than when an
// ? earlier, withdrawn one did. the id deliberately does not, which is what keeps the pushed
// ? frame a replacement on the client — the cost is that the row keeps its place in a list
// ? ordered by id, and a truthful timestamp is worth more than a position nobody can see
func (s *Store) refresh(ctx context.Context, id int64, entityID *int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notifications SET entity_id = ?, created_at = datetime('now') WHERE id = ?`,
		entityID, id)
	if err != nil {
		return fmt.Errorf("refresh notification: %w", err)
	}
	return nil
}

// unreadByGroup is the unread count split per group, which is what lets the groups list
// show a number on each row and the nav tile show their sum
// ! grouped on the resolved group and not on entity_id. entity_id alone would file an
// ! event_created notification under the event's id and invent a group that does not exist
// ? the groups the caller is not a member of are included on purpose: a group_invite is a
// ? notification about a group you are not in yet, and hiding it is hiding the invitation
func (s *Store) unreadByGroup(ctx context.Context, userID int64) ([]GroupUnread, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g, COUNT(*) FROM (
		    SELECT `+groupOf+` AS g
		    FROM notifications n
		    WHERE n.user_id = ? AND n.read_at IS NULL
		)
		-- entity_id carries no foreign key, it is polymorphic by design, so a row can name a
		-- group that is gone. filtering here keeps the sum equal to the numbers the groups
		-- list is able to draw, instead of counting a group nothing can show
		WHERE g IN (SELECT id FROM groups)
		GROUP BY g
		ORDER BY g`, userID)
	if err != nil {
		return nil, fmt.Errorf("unread by group: %w", err)
	}
	defer rows.Close()

	var out []GroupUnread
	for rows.Next() {
		var g GroupUnread
		if err := rows.Scan(&g.GroupID, &g.Unread); err != nil {
			return nil, fmt.Errorf("scan group unread: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// markRead stamps one row read, scoped to the owner so a user cannot mark someone else's,
// stamping only when read_at is still null keeps the original time if called twice
func (s *Store) markRead(ctx context.Context, userID, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notifications SET read_at = datetime('now') WHERE id = ? AND user_id = ? AND read_at IS NULL`,
		id, userID)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	return nil
}

func (s *Store) markAllRead(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notifications SET read_at = datetime('now') WHERE user_id = ? AND read_at IS NULL`, userID)
	if err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}
