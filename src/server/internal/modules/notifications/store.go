package notifications

import (
	"context"
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// insert writes one notification, entityID may be null for a kind that points at nothing,
// the caller has already decided the recipient is not the actor
func (s *Store) insert(ctx context.Context, userID int64, typ string, actorID int64, entityID *int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notifications (user_id, type, actor_id, entity_id) VALUES (?, ?, ?, ?)`,
		userID, typ, actorID, entityID)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// list returns the inbox newest first, joined with the actor for a name and avatar, the
// idx_notifications_inbox index on user_id and id descending backs this read
func (s *Store) list(ctx context.Context, userID int64, limit, offset int) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT n.id, n.type, n.actor_id, u.first_name || ' ' || u.last_name, u.avatar_path,
		        n.entity_id, n.read_at, n.created_at
		 FROM notifications n JOIN users u ON u.id = n.actor_id
		 WHERE n.user_id = ?
		 ORDER BY n.id DESC
		 LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Type, &n.ActorID, &n.ActorName, &n.ActorAvatar, &n.EntityID, &n.ReadAt, &n.CreatedAt); err != nil {
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
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read_at IS NULL`, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("unread count: %w", err)
	}
	return n, nil
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
