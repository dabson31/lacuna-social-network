package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/realtime"
)

// errForbidden is a 403, youre logged in but not allowed. errNotFound is a 404 and also
// covers "this exists but youre not supposed to know that", same as groups does it
var (
	errForbidden = errors.New("not allowed")
	errNotFound  = errors.New("not found")
)

// a chat line, not an essay
//! counted in runes and not bytes, so an emoji costs one character instead of four.
//! a byte limit would cut a message of 500 emojis off halfway through a character and
//! leave broken utf8 in the db
const maxMessageRunes = 2000

const (
	defaultPageSize = 50
	maxPageSize     = 100
)

type Service struct {
	store    *Store
	follows  FollowChecker
	profiles ProfileChecker
	groups   GroupChecker

	// Push is set during wiring, same as follows.Service.Bus and groups.Service.Bus are.
	// a nil Push means messages still get saved and still show up over REST, they just
	// dont arrive live, which is the right way for this to degrade instead of a 500
	Push Pusher

	// this service holds a logger unlike the others, because delivery is best effort:
	// a push that fails has no caller to return an error to, and swallowing it silently
	// is how a chat "mysteriously stops working" with nothing in the logs
	log *slog.Logger
}

func NewService(store *Store, follows FollowChecker, profiles ProfileChecker,
	groups GroupChecker, log *slog.Logger) *Service {
	return &Service{store: store, follows: follows, profiles: profiles, groups: groups, log: log}
}

// ---- direct messages ----

// SendDM saves first, then delivers.
//! that order is the entire reliability story. the row is committed before anybody is told,
//! so a recipient whos offline, or whose socket dies mid push, still has the message
//! waiting for them on the next load. doing it the other way round loses messages
func (s *Service) SendDM(ctx context.Context, senderID int64, in SendDMInput) (DirectMessage, error) {
	content, fields := validContent(in.Content)
	if in.RecipientID <= 0 {
		fields.Add("recipient_id", "a recipient is required")
	}
	if in.RecipientID == senderID {
		fields.Add("recipient_id", "you cannot message yourself")
	}
	if fields.Any() {
		return DirectMessage{}, fields
	}

	if err := s.requireDMAllowed(ctx, senderID, in.RecipientID); err != nil {
		return DirectMessage{}, err
	}

	id, err := s.store.insertDM(ctx, senderID, in.RecipientID, content)
	if err != nil {
		return DirectMessage{}, err
	}
	msg, err := s.store.dmByID(ctx, id)
	if err != nil {
		return DirectMessage{}, err
	}

	// one encode, two recipients. the sender's OTHER tabs need this as much as the
	// recipient does, else sending from your phone leaves your laptop showing an old thread
	//? the same bytes go to both, and the client works out which conversation it belongs to
	//? by taking whichever of sender_id/recipient_id isnt itself. building a personalised
	//? frame per person would buy nothing and cost a marshal per member once a group of
	//? twenty does the same thing
	s.push([]int64{msg.SenderID, msg.RecipientID}, FrameDMMessage, msg)
	return msg, nil
}

// DMHistory uses the same gate as sending. if you cant write to someone you cant read the
// thread either, so a pair who were connected and have since unfollowed lose access to
// their own history. thats the consistent reading of the privacy model even if it feels harsh
func (s *Service) DMHistory(ctx context.Context, me, peer int64, limit int, before int64) ([]DirectMessage, error) {
	if err := s.requireDMAllowed(ctx, me, peer); err != nil {
		return nil, err
	}
	return s.store.dmHistory(ctx, me, peer, clampPage(limit), before)
}

func (s *Service) MarkDMRead(ctx context.Context, me, peer int64) error {
	return s.store.markDMRead(ctx, me, peer)
}

// Inbox needs no gate, its the caller's own list and every row in it comes from a
// relationship theyre already part of
func (s *Service) Inbox(ctx context.Context, me int64) (Inbox, error) {
	direct, err := s.store.conversations(ctx, me)
	if err != nil {
		return Inbox{}, err
	}
	groups, err := s.store.groupConversations(ctx, me)
	if err != nil {
		return Inbox{}, err
	}
	// never nil, the frontend maps over these and null is a crash where [] is an empty list
	if direct == nil {
		direct = []Conversation{}
	}
	if groups == nil {
		groups = []GroupConversation{}
	}
	return Inbox{Direct: direct, Groups: groups}, nil
}

// requireDMAllowed is the subject's rule, and the one the audit checks by logging two
// strangers into two browsers: you can message someone who follows you, someone you follow,
// or anyone whose profile is public. nobody else
//! the checks are ordered by what each one gives away. existence first so a missing user is
//! a clean 404 instead of a confusing 403. public second because its one indexed row and it
//! settles most cases. the follow lookup last since its the only one checking two directions
func (s *Service) requireDMAllowed(ctx context.Context, senderID, recipientID int64) error {
	exists, isPublic, err := s.profiles.ProfileState(ctx, recipientID)
	if err != nil {
		return err
	}
	if !exists {
		return errNotFound
	}
	if isPublic {
		return nil
	}
	connected, err := s.follows.IsConnected(ctx, senderID, recipientID)
	if err != nil {
		return err
	}
	if !connected {
		return errForbidden
	}
	return nil
}

// ---- group messages ----

func (s *Service) SendGroupMessage(ctx context.Context, groupID, senderID int64, in SendGroupMessageInput) (GroupMessage, error) {
	// membership before validation, so a non member cant tell a real group with a bad
	// request apart from a group that doesnt exist
	if err := s.requireGroupMember(ctx, groupID, senderID); err != nil {
		return GroupMessage{}, err
	}
	content, fields := validContent(in.Content)
	if fields.Any() {
		return GroupMessage{}, fields
	}

	id, err := s.store.insertGroupMessage(ctx, groupID, senderID, content)
	if err != nil {
		return GroupMessage{}, err
	}
	msg, err := s.store.groupMessageByID(ctx, id)
	if err != nil {
		return GroupMessage{}, err
	}

	// the sender has obviously read their own message, so moving their watermark now stops
	// it counting against them in their own unread badge
	if err := s.store.setGroupWatermark(ctx, groupID, senderID); err != nil {
		s.log.Error("chat: group watermark", "group", groupID, "user", senderID, "err", err)
	}

	//! membership is read right NOW and not cached in some room object. someone who left
	//! between the last message and this one must not get it, and someone who joined a
	//! second ago must. a cached room list is exactly the bug that leaks a group's chat to
	//! an ex member
	ids, err := s.groups.MemberIDs(ctx, groupID)
	if err != nil {
		// the message is saved, failing to work out the audience is only a delivery problem
		s.log.Error("chat: group fan out", "group", groupID, "err", err)
		return msg, nil
	}
	s.push(ids, FrameGroupMessage, msg)
	return msg, nil
}

func (s *Service) GroupHistory(ctx context.Context, groupID, viewerID int64, limit int, before int64) ([]GroupMessage, error) {
	if err := s.requireGroupMember(ctx, groupID, viewerID); err != nil {
		return nil, err
	}
	return s.store.groupHistory(ctx, groupID, clampPage(limit), before)
}

func (s *Service) MarkGroupRead(ctx context.Context, groupID, userID int64) error {
	if err := s.requireGroupMember(ctx, groupID, userID); err != nil {
		return err
	}
	return s.store.setGroupWatermark(ctx, groupID, userID)
}

// requireGroupMember leans on the GroupChecker interface instead of querying group_members
// itself, so "who counts as a member" keeps one definition over in the groups module
//! a non member gets errNotFound and not errForbidden, we never confirm a group's chat
//! exists to somebody outside it
func (s *Service) requireGroupMember(ctx context.Context, groupID, userID int64) error {
	ok, err := s.groups.IsMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return errNotFound
	}
	return nil
}

// ---- inbound frames ----

// Receive handles one frame a client sent us. it runs on that socket's read goroutine so it
// has to stay cheap: anything slow here delays that client's own pong handling, and pongWait
// later the server hangs up on a perfectly healthy browser
//
// only typing notices come in this way. actual messages go over REST on purpose, a POST
// gets a status code, per field validation and a retry story for free, and an inbound socket
// frame would have to reinvent all three
func (s *Service) Receive(ctx context.Context, userID int64, env realtime.Envelope) {
	switch env.Type {
	case FrameDMTyping:
		var in TypingIn
		if err := json.Unmarshal(env.Payload, &in); err != nil || in.PeerID <= 0 {
			return
		}
		// same gate as sending. without it a stranger could make a "typing..." indicator
		// pop up in your window, which is small but still a hole
		if err := s.requireDMAllowed(ctx, userID, in.PeerID); err != nil {
			return
		}
		s.push([]int64{in.PeerID}, FrameDMTyping, TypingOut{FromUserID: userID})
	}
	//todo: presence.online / presence.offline would slot in here if we ever want it
	// an unknown type is ignored rather than treated as an error, an older client shouldnt
	// get hung up on for sending something a newer server stopped caring about
}

// push is best effort by design. the row is already committed by the time this runs, so a
// client thats offline, slow or gone still gets the message on its next fetch. delivery
// failing must never fail the request that caused it
func (s *Service) push(userIDs []int64, typ string, payload any) {
	if s.Push == nil {
		return
	}
	frame, err := realtime.Encode(typ, payload)
	if err != nil {
		// only reachable through a programming bug since we marshal our own structs
		s.log.Error("chat: encode frame", "type", typ, "err", err)
		return
	}
	s.Push.SendToUsers(userIDs, frame)
}

// validContent trims, rejects empty and caps the length.
//! it does NOT sanitise. content is stored as is in utf8 and escaped at render time by
//! react, stripping characters here is how emojis and apostrophes get mangled and it
//! protects nothing the output layer doesnt already handle
func validContent(raw string) (string, httpx.ValidationErrors) {
	fields := httpx.ValidationErrors{}
	content := strings.TrimSpace(raw)
	switch {
	case content == "":
		fields.Add("content", "a message cannot be empty")
	case utf8.RuneCountInString(content) > maxMessageRunes:
		fields.Add("content", "message is too long")
	}
	return content, fields
}

// clampPage stops a caller from asking for the whole table in one go
func clampPage(limit int) int {
	if limit <= 0 {
		return defaultPageSize
	}
	if limit > maxPageSize {
		return maxPageSize
	}
	return limit
}
