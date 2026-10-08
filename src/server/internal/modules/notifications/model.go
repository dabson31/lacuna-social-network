package notifications

import "time"

// Notification is one row joined with the actor so the ui can show who did it
// EntityID points at the follow, group or event the notification is about
// ReadAt is null until the user opens it, the four type values match the CHECK in the migration
type Notification struct {
	ID          int64      `json:"id"`
	Type        string     `json:"type"`
	ActorID     int64      `json:"actor_id"`
	ActorName   string     `json:"actor_name"`
	ActorAvatar *string    `json:"actor_avatar"`
	EntityID    *int64     `json:"entity_id"`
	ReadAt      *time.Time `json:"read_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Feed is the inbox payload, the unread count is sent alongside so the bell can show a
// badge without a second call, notifications are kept separate from private messages on
// purpose, they are a different type with a different counter
type Feed struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unread_count"`
}
