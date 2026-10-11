package notifications

import "time"

// Notification is one row joined with the actor so the ui can show who did it
// EntityID points at the follow, group or event the notification is about
// ReadAt is null until the user opens it, the four type values match the CHECK in the migration
type Notification struct {
	ID          int64   `json:"id"`
	Type        string  `json:"type"`
	ActorID     int64   `json:"actor_id"`
	ActorName   string  `json:"actor_name"`
	ActorAvatar *string `json:"actor_avatar"`
	EntityID    *int64  `json:"entity_id"`
	// GroupID is which group this notification belongs to, null for a kind that belongs to
	// no group. it is derived and not stored, because EntityID already points at the group
	// for an invite or a join request, and at an event for event_created
	// ! this is what lets the ui count notifications per group. without it the three group
	// ! kinds are indistinguishable from each other once they are in a list, since entity_id
	// ! means a different table per kind and nothing on the row says which
	GroupID   *int64     `json:"group_id"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// GroupUnread is one group's share of the unread count, so the groups list can show a
// number per group and the nav can show their sum
// ? counted in sql rather than derived from Feed.Notifications, because that list is one
// ? page of the inbox: deriving from it would undercount the moment somebody has more
// ? unread notifications than the page holds
type GroupUnread struct {
	GroupID int64 `json:"group_id"`
	Unread  int   `json:"unread"`
}

// Feed is the inbox payload, the unread count is sent alongside so the bell can show a
// badge without a second call, notifications are kept separate from private messages on
// purpose, they are a different type with a different counter
type Feed struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unread_count"`
	// the same count split by group, for the groups list and the groups tile. it only ever
	// holds the group kinds, so its sum is never larger than UnreadCount
	GroupUnread []GroupUnread `json:"group_unread"`
}

// the frame type this module owns, mirrored in src/shared/constants/ws.ts
// ! this is the upgrade from "fetch on load" to live: the same row that gets stored also
// ! goes down the socket, so the bell lights up without a refresh
const FrameNotificationNew = "notification.new"
