package chat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var errNoRow = errors.New("no such message")

// Store owns direct_messages and group_messages.
//
// the inbox queries down below also read follows, users, groups and group_members. thats a
// read of columns, not a module import, the same thing follows/store.go does when it reads
// users.is_public
//! the line this module holds is that no rule gets decided from those reads. "members only"
//! and "you have to be connected" both go through the interfaces in ports.go, so those
//! rules keep exactly one owner. reading rows for display is fine, deciding with them isnt
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ---- direct messages ----

// the shared select, so a pushed frame and a history row can never drift apart
const selectDM = `
	SELECT m.id, m.sender_id, m.recipient_id, m.content, m.created_at, m.read_at,
	       u.first_name || ' ' || u.last_name, u.avatar_path
	FROM direct_messages m JOIN users u ON u.id = m.sender_id`

func scanDM(row interface{ Scan(...any) error }) (DirectMessage, error) {
	var m DirectMessage
	err := row.Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.Content, &m.CreatedAt,
		&m.ReadAt, &m.SenderName, &m.SenderAvatar)
	return m, err
}

func (s *Store) insertDM(ctx context.Context, senderID, recipientID int64, content string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO direct_messages (sender_id, recipient_id, content) VALUES (?, ?, ?)`,
		senderID, recipientID, content)
	if err != nil {
		return 0, fmt.Errorf("insert dm: %w", err)
	}
	return res.LastInsertId()
}

// dmByID reads back the row we just wrote, so the caller gets the server's created_at and
// the sender's name instead of guessing at either one
func (s *Store) dmByID(ctx context.Context, id int64) (DirectMessage, error) {
	m, err := scanDM(s.db.QueryRowContext(ctx, selectDM+` WHERE m.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return DirectMessage{}, errNoRow
	}
	if err != nil {
		return DirectMessage{}, fmt.Errorf("dm by id: %w", err)
	}
	return m, nil
}

// dmHistory gives back one page of a conversation, oldest first.
//
// paging is by message id and not OFFSET, because messages keep arriving while youre
// scrolling. with an offset the page boundary slides underneath you and you get shown the
// same message twice. before = 0 means start from the newest
//
// the query sorts DESC so LIMIT grabs the newest N, then we flip the slice in go. a chat
// draws oldest at the top but you page backwards from the bottom, so one of the two has to
// reverse and doing it in sql would need a subquery for nothing. idx_dm_pair backs the sort
//! named params (:me) instead of ?, because me shows up four times here and three in
//! conversations below. a miscounted ? doesnt error, it just silently answers the wrong
//! question, which is the worst kind of bug. mattn/go-sqlite3 supports them via sql.Named
func (s *Store) dmHistory(ctx context.Context, me, peer int64, limit int, before int64) ([]DirectMessage, error) {
	rows, err := s.db.QueryContext(ctx, selectDM+`
		WHERE ((m.sender_id = :me AND m.recipient_id = :peer)
		    OR (m.sender_id = :peer AND m.recipient_id = :me))
		  AND (:before = 0 OR m.id < :before)
		ORDER BY m.id DESC
		LIMIT :limit`,
		sql.Named("me", me), sql.Named("peer", peer),
		sql.Named("before", before), sql.Named("limit", limit))
	if err != nil {
		return nil, fmt.Errorf("dm history: %w", err)
	}
	defer rows.Close()

	var out []DirectMessage
	for rows.Next() {
		m, err := scanDM(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dm: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reverse(out), nil
}

// markDMRead stamps everything this one peer sent me.
// scoped to recipient_id = me so nobody can mark somebody elses messages read, and to
// read_at IS NULL so reopening a chat keeps the original time instead of bumping it
//? idx_dm_unread is a partial index holding only the unread rows, which is what makes this
//? cheap no matter how much history piles up
func (s *Store) markDMRead(ctx context.Context, me, peer int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE direct_messages SET read_at = datetime('now')
		  WHERE recipient_id = :me AND sender_id = :peer AND read_at IS NULL`,
		sql.Named("me", me), sql.Named("peer", peer))
	if err != nil {
		return fmt.Errorf("mark dm read: %w", err)
	}
	return nil
}

// conversations lists everyone the caller can chat with, plus the last line and the unread count.
//
// peers is a UNION of four things: people you follow, people who follow you, and both
// directions of existing history. the follow halves are why a brand new mutual follow turns
// up in the sidebar with nothing said yet, and the history halves are why a chat with a
// public stranger doesnt disappear if neither of you ever follows the other
//? UNION and not UNION ALL, because UNION dedupes and someone can easily be in two or three
//? of those four lists at once
//
// newest grabs each peer's last message id once, then the LEFT JOIN pulls that single row's
// content. doing it as two correlated subqueries would walk the same index twice to answer
// one question
//! "ORDER BY last_id IS NULL, last_id DESC" is sqlite's way of saying NULLS LAST. busiest
//! chat at the top, people youve never messaged at the bottom in name order
func (s *Store) conversations(ctx context.Context, me int64) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH peers(id) AS (
		    SELECT followee_id FROM follows WHERE follower_id = :me AND status = 'accepted'
		    UNION
		    SELECT follower_id FROM follows WHERE followee_id = :me AND status = 'accepted'
		    UNION
		    SELECT recipient_id FROM direct_messages WHERE sender_id = :me
		    UNION
		    SELECT sender_id    FROM direct_messages WHERE recipient_id = :me
		),
		newest AS (
		    SELECT p.id AS peer_id,
		           (SELECT m.id FROM direct_messages m
		             WHERE (m.sender_id = :me AND m.recipient_id = p.id)
		                OR (m.sender_id = p.id AND m.recipient_id = :me)
		             ORDER BY m.id DESC LIMIT 1) AS last_id
		    FROM peers p
		)
		SELECT u.id, u.first_name, u.last_name, u.avatar_path,
		       COALESCE(dm.content, ''), dm.created_at,
		       (SELECT COUNT(*) FROM direct_messages m2
		         WHERE m2.sender_id = u.id AND m2.recipient_id = :me AND m2.read_at IS NULL)
		FROM newest n
		JOIN users u ON u.id = n.peer_id
		LEFT JOIN direct_messages dm ON dm.id = n.last_id
		WHERE u.id <> :me
		ORDER BY n.last_id IS NULL, n.last_id DESC, u.first_name`,
		sql.Named("me", me))
	if err != nil {
		return nil, fmt.Errorf("conversations: %w", err)
	}
	defer rows.Close()

	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.PeerID, &c.FirstName, &c.LastName, &c.AvatarPath,
			&c.LastMessage, &c.LastAt, &c.Unread); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---- group messages ----

const selectGroupMessage = `
	SELECT m.id, m.group_id, m.sender_id, m.content, m.created_at,
	       u.first_name || ' ' || u.last_name, u.avatar_path
	FROM group_messages m JOIN users u ON u.id = m.sender_id`

func scanGroupMessage(row interface{ Scan(...any) error }) (GroupMessage, error) {
	var m GroupMessage
	err := row.Scan(&m.ID, &m.GroupID, &m.SenderID, &m.Content, &m.CreatedAt,
		&m.SenderName, &m.SenderAvatar)
	return m, err
}

func (s *Store) insertGroupMessage(ctx context.Context, groupID, senderID int64, content string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO group_messages (group_id, sender_id, content) VALUES (?, ?, ?)`,
		groupID, senderID, content)
	if err != nil {
		return 0, fmt.Errorf("insert group message: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) groupMessageByID(ctx context.Context, id int64) (GroupMessage, error) {
	m, err := scanGroupMessage(s.db.QueryRowContext(ctx, selectGroupMessage+` WHERE m.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return GroupMessage{}, errNoRow
	}
	if err != nil {
		return GroupMessage{}, fmt.Errorf("group message by id: %w", err)
	}
	return m, nil
}

// groupHistory is dmHistory's shape but for a room, same id cursor and same DESC then flip
func (s *Store) groupHistory(ctx context.Context, groupID int64, limit int, before int64) ([]GroupMessage, error) {
	rows, err := s.db.QueryContext(ctx, selectGroupMessage+`
		WHERE m.group_id = :gid AND (:before = 0 OR m.id < :before)
		ORDER BY m.id DESC
		LIMIT :limit`,
		sql.Named("gid", groupID), sql.Named("before", before), sql.Named("limit", limit))
	if err != nil {
		return nil, fmt.Errorf("group history: %w", err)
	}
	defer rows.Close()

	var out []GroupMessage
	for rows.Next() {
		m, err := scanGroupMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan group message: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reverse(out), nil
}

// setGroupWatermark moves this member's last_read_message_id up to the newest message in
// the room. COALESCE covers a group nobody has written in yet, and the status check means a
// pending invite cant be used to mark a room read
func (s *Store) setGroupWatermark(ctx context.Context, groupID, userID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE group_members
		    SET last_read_message_id =
		        COALESCE((SELECT MAX(id) FROM group_messages WHERE group_id = :gid), 0)
		  WHERE group_id = :gid AND user_id = :uid AND status = 'member'`,
		sql.Named("gid", groupID), sql.Named("uid", userID))
	if err != nil {
		return fmt.Errorf("set group watermark: %w", err)
	}
	return nil
}

// groupConversations is the group half of the inbox: every room the caller is a member of,
// its last line, and how many messages turned up past their watermark
//! "m.sender_id <> :me" in the count is what stops your own message counting as unread in
//! the split second between sending it and the watermark moving
func (s *Store) groupConversations(ctx context.Context, me int64) ([]GroupConversation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.title,
		       COALESCE(lm.content, ''),
		       COALESCE(lu.first_name || ' ' || lu.last_name, ''),
		       lm.created_at,
		       (SELECT COUNT(*) FROM group_messages m
		         WHERE m.group_id = g.id
		           AND m.id > mem.last_read_message_id
		           AND m.sender_id <> :me)
		FROM group_members mem
		JOIN groups g ON g.id = mem.group_id
		LEFT JOIN group_messages lm
		       ON lm.id = (SELECT MAX(id) FROM group_messages WHERE group_id = g.id)
		LEFT JOIN users lu ON lu.id = lm.sender_id
		WHERE mem.user_id = :me AND mem.status = 'member'
		ORDER BY lm.id IS NULL, lm.id DESC, g.title`,
		sql.Named("me", me))
	if err != nil {
		return nil, fmt.Errorf("group conversations: %w", err)
	}
	defer rows.Close()

	var out []GroupConversation
	for rows.Next() {
		var c GroupConversation
		if err := rows.Scan(&c.GroupID, &c.Title, &c.LastMessage, &c.LastSender,
			&c.LastAt, &c.Unread); err != nil {
			return nil, fmt.Errorf("scan group conversation: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// reverse flips a slice in place and hands it back, so both history functions can end on
// one line. generic because dms and group messages are different types doing the same thing
//? the swap walks two pointers at each other until they meet, which is why it stops at i < j
func reverse[T any](s []T) []T {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}
