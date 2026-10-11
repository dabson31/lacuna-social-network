package notifications

import (
	"context"
	"log/slog"

	"social/src/server/internal/platform/events"
	"social/src/server/internal/platform/realtime"
)

type Service struct {
	store *Store
	// Push is wired at startup the same way the buses are. nil means notifications are
	// still stored and still turn up on page load, the bell just doesnt light up until a
	// refresh, which is exactly the behaviour this is upgrading
	Push Pusher
	// the logger is here because a failed push has no caller to return an error to, the
	// row already exists and the request that caused it is long gone
	log *slog.Logger
}

func NewService(store *Store, log *slog.Logger) *Service {
	return &Service{store: store, log: log}
}

// Record is the event bus subscriber, it turns an event into a stored notification, it
// runs in the bus goroutine after the action that caused it has already committed, so a
// failure here cannot undo that action
// ie, we never notify someone about their own actionS
func (s *Service) Record(ctx context.Context, e events.Event) error {
	if e.RecipientID == e.ActorID {
		return nil
	}
	id, err := s.write(ctx, e)
	if err != nil {
		return err
	}

	//! the row is written FIRST so an offline user still finds it on next login. the push
	//! is a live upgrade sitting on top of a durable row and never a replacement for it,
	//! which is why everything below logs and returns nil: the notification exists either
	//! way, the only thing that got missed is the doorbell
	if s.Push == nil {
		return nil
	}
	n, err := s.store.byID(ctx, id)
	if err != nil {
		s.log.Error("notifications: read back for push", "id", id, "err", err)
		return nil
	}
	frame, err := realtime.Encode(FrameNotificationNew, n)
	if err != nil {
		s.log.Error("notifications: encode frame", "id", id, "err", err)
		return nil
	}
	s.Push.SendToUser(e.RecipientID, frame)
	return nil
}

// write stores the event as a notification and returns the row's id, collapsing a repeat
// into the row already waiting instead of adding a second one
// ! the pile-up this prevents: request to follow, cancel, request again, and the recipient
// ! collected one notification per attempt — every one of them except the newest pointing at a
// ! follows row that no longer existed, so none of them could be accepted and none could be
// ! dismissed. one row per waiting request is what the reader actually means by "a request"
// ! the id is kept on purpose when a row is reused. the frontend dedupes a pushed frame by id,
// ! so refreshing in place makes an open Pending panel REPLACE the row it is showing rather
// ! than draw a second one, with no client side rule mirroring this one
func (s *Service) write(ctx context.Context, e events.Event) (int64, error) {
	//! the entity is part of what makes two notifications the same thing, EXCEPT for a follow
	//! request. there the entity IS the request, and cancelling and resending makes a new
	//! follows row every time, so matching on it would never collapse anything. for the other
	//! three kinds the entity is the subject — a group, an event — and two different subjects
	//! are two different notifications: collapsing on actor alone would silently drop the
	//! notification for a second event raised in the same group by the same person
	matchEntity := e.Kind != events.FollowRequested

	//? read-then-write, not atomic, and that is accounted for rather than ignored: two bus
	//? goroutines for the same pair can both miss and both insert. the reader still sees one,
	//? because `live` hides whichever row is left pointing at a follows row that is gone. a
	//? partial unique index would close the window and costs a migration for a duplicate
	//? nobody can see
	existing, err := s.store.findUnread(ctx, e.RecipientID, e.Kind, e.ActorID, e.EntityID, matchEntity)
	if err != nil {
		return 0, err
	}
	if existing != 0 {
		return existing, s.store.refresh(ctx, existing, e.EntityID)
	}
	return s.store.insert(ctx, e.RecipientID, e.Kind, e.ActorID, e.EntityID)
}

// Feed returns the inbox plus the unread count in one payload, the count is read after
// the list so a page can draw the bell badge without a second request
func (s *Service) Feed(ctx context.Context, userID int64, limit, offset int) (Feed, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	list, err := s.store.list(ctx, userID, limit, offset)
	if err != nil {
		return Feed{}, err
	}
	count, err := s.store.unreadCount(ctx, userID)
	if err != nil {
		return Feed{}, err
	}
	// the same unread count split per group, so the groups list can draw a number on each
	// row and the nav can draw their sum, both off this one request
	perGroup, err := s.store.unreadByGroup(ctx, userID)
	if err != nil {
		return Feed{}, err
	}
	if list == nil {
		list = []Notification{}
	}
	if perGroup == nil {
		perGroup = []GroupUnread{}
	}
	return Feed{Notifications: list, UnreadCount: count, GroupUnread: perGroup}, nil
}

func (s *Service) MarkRead(ctx context.Context, userID, id int64) error {
	return s.store.markRead(ctx, userID, id)
}

func (s *Service) MarkAllRead(ctx context.Context, userID int64) error {
	return s.store.markAllRead(ctx, userID)
}
