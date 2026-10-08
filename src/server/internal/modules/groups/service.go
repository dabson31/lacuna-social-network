package groups

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"social/src/server/internal/platform/events"
	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/storage"
)

// the authorization errors every group action can raise, the handler maps them to codes,
// errForbidden is a 403 the caller is logged in but not allowed, errNotFound hides a thing
// that exists from someone who may not see it
var (
	errForbidden = errors.New("not allowed")
	errNotFound  = errors.New("not found")
)

type Service struct {
	store   *Store
	uploads *storage.Store
	// Bus is optional and set during wiring, group notifications go through it, a nil Bus
	// means no notifications are emitted
	Bus *events.Bus
}

func NewService(store *Store, uploads *storage.Store) *Service {
	return &Service{store: store, uploads: uploads}
}

func (s *Service) Create(ctx context.Context, creatorID int64, in CreateGroupInput) (Group, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	if in.JoinPolicy == "" {
		in.JoinPolicy = "request"
	}
	if fields := validateGroup(in); fields.Any() {
		return Group{}, fields
	}
	id, err := s.store.createGroup(ctx, creatorID, in)
	if err != nil {
		return Group{}, err
	}
	return s.store.byID(ctx, creatorID, id)
}

func (s *Service) Browse(ctx context.Context, viewerID int64) ([]Group, error) {
	return s.store.browse(ctx, viewerID)
}

// Get returns the group, the member list is attached only for members, a non member
// still gets the header and the count so they can see it exists and ask to join
func (s *Service) Get(ctx context.Context, viewerID, groupID int64) (GroupDetail, error) {
	g, err := s.store.byID(ctx, viewerID, groupID)
	if err != nil {
		return GroupDetail{}, err
	}
	detail := GroupDetail{Group: g}
	if g.MyStatus == StatusMember {
		members, err := s.store.members(ctx, groupID)
		if err != nil {
			return GroupDetail{}, err
		}
		detail.Members = members
	}
	return detail, nil
}

// requireMember is the gate at the top of every members only action, it returns the
// caller's role too so a later check can tell the creator apart from a plain member
func (s *Service) requireMember(ctx context.Context, groupID, userID int64) (role string, err error) {
	status, role, err := s.store.membership(ctx, groupID, userID)
	if err != nil {
		return "", err
	}
	if status != StatusMember {
		// not a member reads as not found, we never confirm a group's contents to outsiders
		return "", errNotFound
	}
	return role, nil
}

// ---- group content ----

func (s *Service) CreatePost(ctx context.Context, groupID, authorID int64, in GroupPostInput) (GroupPost, error) {
	if _, err := s.requireMember(ctx, groupID, authorID); err != nil {
		return GroupPost{}, err
	}
	in.Content = strings.TrimSpace(in.Content)
	if in.Content == "" && in.Image == nil {
		return GroupPost{}, httpx.ValidationErrors{"content": "a post needs text or an image"}
	}

	var imagePath *string
	if in.Image != nil {
		name, err := s.uploads.Save(in.Image)
		if err != nil {
			return GroupPost{}, err
		}
		imagePath = &name
	}
	id, err := s.store.addPost(ctx, groupID, authorID, in.Content, imagePath)
	if err != nil {
		if imagePath != nil {
			_ = s.uploads.Delete(*imagePath)
		}
		return GroupPost{}, err
	}
	return findPost(s.store.posts(ctx, groupID))(id), nil
}

func (s *Service) Posts(ctx context.Context, groupID, viewerID int64) ([]GroupPost, error) {
	if _, err := s.requireMember(ctx, groupID, viewerID); err != nil {
		return nil, err
	}
	return s.store.posts(ctx, groupID)
}

func (s *Service) CreateComment(ctx context.Context, postID, authorID int64, in GroupPostInput) (GroupComment, error) {
	// a comment belongs to a post, and the post belongs to a group, so the commenter has
	// to be a member of that same group, we resolve the group from the post first
	groupID, err := s.store.postGroup(ctx, postID)
	if err != nil {
		return GroupComment{}, errNotFound
	}
	if _, err := s.requireMember(ctx, groupID, authorID); err != nil {
		return GroupComment{}, err
	}
	in.Content = strings.TrimSpace(in.Content)
	if in.Content == "" && in.Image == nil {
		return GroupComment{}, httpx.ValidationErrors{"content": "a comment needs text or an image"}
	}

	var imagePath *string
	if in.Image != nil {
		name, err := s.uploads.Save(in.Image)
		if err != nil {
			return GroupComment{}, err
		}
		imagePath = &name
	}
	id, err := s.store.addComment(ctx, postID, authorID, in.Content, imagePath)
	if err != nil {
		if imagePath != nil {
			_ = s.uploads.Delete(*imagePath)
		}
		return GroupComment{}, err
	}
	for _, c := range dropErr(s.store.comments(ctx, postID)) {
		if c.ID == id {
			return c, nil
		}
	}
	return GroupComment{}, nil
}

func (s *Service) Comments(ctx context.Context, postID, viewerID int64) ([]GroupComment, error) {
	groupID, err := s.store.postGroup(ctx, postID)
	if err != nil {
		return nil, errNotFound
	}
	if _, err := s.requireMember(ctx, groupID, viewerID); err != nil {
		return nil, err
	}
	return s.store.comments(ctx, postID)
}

// ---- events ----

func (s *Service) CreateEvent(ctx context.Context, groupID, creatorID int64, in CreateEventInput) (Event, error) {
	if _, err := s.requireMember(ctx, groupID, creatorID); err != nil {
		return Event{}, err
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)

	fields := httpx.ValidationErrors{}
	if in.Title == "" {
		fields.Add("title", "title is required")
	}
	if in.Description == "" {
		fields.Add("description", "description is required")
	}
	// the time comes in as an RFC 3339 string, we parse it to be sure it is a real instant
	// and store it back in a fixed format so every row sorts the same way
	t, err := time.Parse(time.RFC3339, in.EventTime)
	if err != nil {
		fields.Add("event_time", "event time must be an RFC 3339 timestamp")
	}
	if fields.Any() {
		return Event{}, fields
	}

	id, err := s.store.addEvent(ctx, groupID, creatorID, in, t.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return Event{}, err
	}

	// every member except the creator is told an event was created, we read the member
	// list now rather than caching it because membership changes, one event per recipient
	if s.Bus != nil {
		if ids, err := s.store.memberIDs(ctx, groupID, creatorID); err == nil {
			eventID := id
			for _, uid := range ids {
				s.Bus.Publish(ctx, events.Event{
					Kind:        events.EventCreated,
					RecipientID: uid,
					ActorID:     creatorID,
					EntityID:    &eventID,
				})
			}
		}
	}

	for _, e := range dropErrEvents(s.store.events(ctx, groupID, creatorID)) {
		if e.ID == id {
			return e, nil
		}
	}
	return Event{}, nil
}

func (s *Service) Events(ctx context.Context, groupID, viewerID int64) ([]Event, error) {
	if _, err := s.requireMember(ctx, groupID, viewerID); err != nil {
		return nil, err
	}
	return s.store.events(ctx, groupID, viewerID)
}

// Respond records going or not_going, only a member of the event's group may answer,
// the store upserts so a second answer replaces the first
func (s *Service) Respond(ctx context.Context, eventID, userID int64, in RespondInput) error {
	if in.Response != "going" && in.Response != "not_going" {
		return httpx.ValidationErrors{"response": "response must be going or not_going"}
	}
	groupID, err := s.store.eventGroup(ctx, eventID)
	if err != nil {
		return errNotFound
	}
	if _, err := s.requireMember(ctx, groupID, userID); err != nil {
		return err
	}
	return s.store.respond(ctx, eventID, userID, in.Response)
}

func validateGroup(in CreateGroupInput) httpx.ValidationErrors {
	fields := httpx.ValidationErrors{}
	if in.Title == "" {
		fields.Add("title", "title is required")
	} else if utf8.RuneCountInString(in.Title) > 100 {
		fields.Add("title", "title is too long")
	}
	if in.Description == "" {
		fields.Add("description", "description is required")
	} else if utf8.RuneCountInString(in.Description) > 1000 {
		fields.Add("description", "description is too long")
	}
	if in.JoinPolicy != "request" && in.JoinPolicy != "open" {
		fields.Add("join_policy", "join policy must be request or open")
	}
	return fields
}

// findPost returns a lookup over a freshly listed slice, it lets the create path hand back
// the stored row including the author name without a second dedicated query
func findPost(list []GroupPost, _ error) func(int64) GroupPost {
	return func(id int64) GroupPost {
		for _, p := range list {
			if p.ID == id {
				return p
			}
		}
		return GroupPost{}
	}
}

func dropErr(c []GroupComment, _ error) []GroupComment { return c }
func dropErrEvents(e []Event, _ error) []Event         { return e }
