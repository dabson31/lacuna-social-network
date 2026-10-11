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
	//! nothing was written, and that is a result rather than a failure: the thing this would
	//! have announced stopped being true while the event was in flight. a follow request
	//! withdrawn before its own notification reached the table is the case that happens, and
	//! the id MUST NOT be read back — an insert that wrote no row still reports the last id
	//! this connection did write, which belongs to somebody else
	if id == 0 {
		return nil
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

// Forget is the other event bus subscriber: it deletes the notification an event has just
// made untrue, and tells the recipient's open tabs to drop it.
//
// ! the mirror of Record, and a separate subscriber rather than a branch inside it, because
// ! the two do opposite things to the table and share none of the collapse rules. the only
// ! kind wired to it today is a withdrawn follow request
//
// ! the bus kind is NOT the notification type here. a cancel is a retraction of a
// ! `follow_request`, so the row being deleted is that type — which is the one place in this
// ! module where the two strings differ, and the reason the mapping is explicit below rather
// ! than e.Kind being used directly the way write uses it
//
// ? it runs in a bus goroutine after the follows row is already gone, so there is nothing to
// ? roll back and nothing to fail: an error here is logged by the bus and the reader simply
// ? keeps a row `live` was already hiding
func (s *Service) Forget(ctx context.Context, e events.Event) error {
	typ, ok := retracts[e.Kind]
	if !ok {
		return nil
	}

	ids, err := s.store.deleteByEntity(ctx, e.RecipientID, typ, e.ActorID, e.EntityID)
	if err != nil {
		return err
	}
	if len(ids) == 0 || s.Push == nil {
		return nil
	}

	//! the push is best effort and the delete is not, the same split Record uses: the row is
	//! gone whether or not anyone is listening, and a reader who was offline simply never
	//! sees it again. so everything below logs and returns nil
	frame, err := realtime.Encode(FrameNotificationGone, Gone{IDs: ids})
	if err != nil {
		s.log.Error("notifications: encode gone frame", "ids", ids, "err", err)
		return nil
	}
	s.Push.SendToUser(e.RecipientID, frame)
	return nil
}

// retracts maps a retraction event to the notification type it deletes.
// ? a map rather than a switch so adding a second retraction is one line here and one
// ? Subscribe in the router, with no branch for a reader to miss
var retracts = map[string]string{
	events.FollowRequestCancelled: events.FollowRequested,
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
	//! a follow_request is the one kind whose entity can be deleted behind the notification's
	//! back, by the sender, at any moment — including while this very event is still queued.
	//! so it is the one kind that matches on the actor rather than the entity, AND the one
	//! kind whose write is conditional on the request still being there
	isRequest := e.Kind == events.FollowRequested

	//? read-then-write, not atomic, and that is accounted for rather than ignored: two bus
	//? goroutines for the same pair can both miss and both insert. the reader still sees one,
	//? because `live` hides whichever row is left pointing at a follows row that is gone. a
	//? partial unique index would close the window and costs a migration for a duplicate
	//? nobody can see
	existing, err := s.store.findUnread(ctx, e.RecipientID, e.Kind, e.ActorID, e.EntityID, !isRequest)
	if err != nil {
		return 0, err
	}
	if existing != 0 {
		//? a refused refresh reports 0 and not the row's id, so nothing is pushed. the row is
		//? still sitting there pointing at the previous request, which `live` already hides
		//? and the next request from this actor recycles
		moved, err := s.store.refresh(ctx, existing, e.EntityID, isRequest)
		if err != nil || !moved {
			return 0, err
		}
		return existing, nil
	}
	return s.store.insert(ctx, e.RecipientID, e.Kind, e.ActorID, e.EntityID, isRequest)
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
