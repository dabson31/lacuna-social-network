package events

import (
	"context"
	"log/slog"
	"sync"
)

// these kinds are published by features when something notifiable happens, the strings
// are also the notification type stored in the db, so the subscriber inserts without a
// translation step, they line up with the CHECK on notifications type
const (
	FollowRequested = "follow_request"
	GroupInvited = "group_invite"
	GroupJoinRequested = "group_join_request"
	EventCreated = "event_created"
)

// Event is an action that happened; Like a folow request, or a group invitation,
// Kind is the type of event that occured
// RecipientID is who should hear about it,
// ActorID is who caused it,
// EntityID points to the row it is about; a follow, a group, or an event
type Event struct {
	Kind 		string
	RecipientID int64
	ActorID 	int64
	EntityID 	*int64
}

// Handler reacts to an event, it runs its own goroutine.
// Therefore it must not assume the request is still around
type Handler func(ctx context.Context, e Event)

// Bus is a tiny in process publish subscribe, it is how a feature tells another feature
// something happened without importing it, follows publishes and notifications listens, 
// neither knows about the other; 
// Think of it as a post office, it makes sure everyone gets their post;
// (every event gets assigned a handler)
type Bus struct {
	mu sync.RWMutex
	subs map[string][]Handler
	log *slog.Logger
}

// NewBus initializes a Bus to handle events triggered by users
func NewBus(log *slog.Logger) *Bus {
	return &Bus{subs: make(map[string][]Handler), log: log}
}

// Subscribe registers a handler for one kind, called once at startup while wriring the app
// It adds a function that satisifes type Handler to the map for this specific kind of event
func (b *Bus) Subscribe(kind string, h Handler){
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[kind] = append(b.subs[kind], h)
}

// Publish is fire and forget, it never blocks the caller and never returns an error,a
// notification that fails to write must not roll back the follow that triggered it, each
// handler runs in its own goroutine with its own recover so one panic cannot take the
// process down, we strip cancellation from the context because the request may have
// already returned by the time the handler writes its row, yet we keep its values
func (b *Bus) Publish(ctx context.Context, e Event){
	b.mu.Lock()
	handlers := b.subs[e.Kind]
	b.mu.Unlock()

	// This creates a clone of the context and keeps the useful values, but strips out the cancellation signal 
	ctx = context.WithoutCancel(ctx)
	for _, h := range handlers{
		// This prevents loop variable capture bugs; Each goroutine gets its own copy of the handler
		h := h 
		go func() {
			// defer func() schedules the code to be executed before the go routine returns;
			// Used for panic recovery in this case
			defer func(){ 
				if v := recover(); v != nil{
					b.log.Error("event handler panic","kind", e.Kind, "err", v)
				}
			}()
			h(ctx, e)
		}() // FYI: The Brackets at the end immediately executes the anonymous function
	}
}