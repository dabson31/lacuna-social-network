package notifications

import (
	"context"

	"social/src/server/internal/platform/events"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

// Record is the event bus subscriber, it turns an event into a stored notification, it
// runs in the bus goroutine after the action that caused it has already committed, so a
// failure here cannot undo that action
// ie, we never notify someone about their own actionS
func (s *Service) Record(ctx context.Context, e events.Event) error {
	if e.RecipientID == e.ActorID {
		return nil
	}
	if err := s.store.insert(ctx, e.RecipientID, e.Kind, e.ActorID, e.EntityID); err != nil {
		return err
	}

	// the row is written first so an offline user still sees it on next login, this is
	// where a notification new frame will be pushed to the recipient over the websocket
	// once the realtime hub exists, delivery is best effort on top of the stored row

	return nil
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
	if list == nil {
		list = []Notification{}
	}
	return Feed{Notifications: list, UnreadCount: count}, nil
}

func (s *Service) MarkRead(ctx context.Context, userID, id int64) error {
	return s.store.markRead(ctx, userID, id)
}

func (s *Service) MarkAllRead(ctx context.Context, userID int64) error {
	return s.store.markAllRead(ctx, userID)
}
