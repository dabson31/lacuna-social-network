package follows

import (
	"context"
	"errors"

	"social/src/server/internal/platform/events"
)

type Service struct {
	store *Store
	Bus   *events.Bus
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

// Follow starts following target, a public target is accepted straight away while a
// private one becomes a pending request, the returned status is which of the two happened
// ! a check has been added for: the caller cannot follow themselves and the target has to exist
func (s *Service) Follow(ctx context.Context, followerID, targetID int64) (FollowResult, error) {
	if followerID == targetID {
		return FollowResult{}, errSelf
	}
	isPublic, err := s.store.targetState(ctx, targetID)
	if err != nil {
		return FollowResult{}, err
	}

	status := "pending"
	if isPublic {
		status = "accepted"
	}
	followID, err := s.store.insert(ctx, followerID, targetID, status)
	if err != nil {
		return FollowResult{}, err
	}

	// a pending request against a private account is where a follow requested
	// notification will be published once the notifications feature exists, a public
	// auto follow needs none because there is nothing to approve
	if status == "pending" && s.Bus != nil {
		s.Bus.Publish(ctx, events.Event{
			Kind:        events.FollowRequested,
			RecipientID: targetID,
			ActorID:     followerID,
			EntityID:    &followID,
		})
	}

	return FollowResult{Status: status}, nil
}

// Unfollow removes the row in either state, so it also cancels a request you sent,
// nothing deleted means you were not following in the first place
//
// ! cancelling a PENDING request also retracts the notification it raised. the request is
// ! gone, so a notice still offering an accept points at nothing, cannot be acted on and
// ! cannot be cleared — request, cancel, request again and the recipient collected one dead
// ! row per attempt. the retraction is what stops that pile forming in the first place
//
// ? only pending. unfollowing an ACCEPTED follow deletes the same table's row, but the
// ? request behind it was answered, and that answer is the reader's history rather than a
// ? loose end. publishing on both would quietly delete a resolved notification every time
// ? somebody unfollowed
func (s *Service) Unfollow(ctx context.Context, followerID, targetID int64) error {
	followID, status, err := s.store.remove(ctx, followerID, targetID)
	if err != nil {
		return err
	}

	if status == "pending" && s.Bus != nil {
		s.Bus.Publish(ctx, events.Event{
			Kind:        events.FollowRequestCancelled,
			RecipientID: targetID,
			ActorID:     followerID,
			// the follows row that just stopped existing. the subscriber deletes the
			// notification pointing AT IT and no other, which is what makes a cancel and an
			// instantly resent request safe to interleave
			EntityID: &followID,
		})
	}
	return nil
}

// Accept turns a pending request into an accepted follow, only the person the request
// was aimed at may accept it, anything else is treated as if the request does not exist
func (s *Service) Accept(ctx context.Context, viewerID, followID int64) error {
	_, followeeID, status, err := s.store.requestParties(ctx, followID)
	if err != nil {
		return err
	}
	if followeeID != viewerID || status != "pending" {
		return errNoRequest
	}
	return s.store.setStatus(ctx, followID, "accepted")
}

// Decline deletes the pending request, same ownership check as accept
func (s *Service) Decline(ctx context.Context, viewerID, followID int64) error {
	_, followeeID, status, err := s.store.requestParties(ctx, followID)
	if err != nil {
		return err
	}
	if followeeID != viewerID || status != "pending" {
		return errNoRequest
	}
	return s.store.deleteByID(ctx, followID)
}

func (s *Service) IncomingRequests(ctx context.Context, viewerID int64) ([]RequestItem, error) {
	return s.store.incoming(ctx, viewerID)
}

// Followers returns owner's followers when the viewer is allowed to see them, the same
// visibility rule as the profile applies, you can look when it is you, or owner is public,
// or you are an accepted follower, otherwise it reads as not found
func (s *Service) Followers(ctx context.Context, viewerID, ownerID int64) ([]UserCard, error) {
	if err := s.canView(ctx, viewerID, ownerID); err != nil {
		return nil, err
	}
	return s.store.listFollowers(ctx, ownerID)
}

func (s *Service) Following(ctx context.Context, viewerID, ownerID int64) ([]UserCard, error) {
	if err := s.canView(ctx, viewerID, ownerID); err != nil {
		return nil, err
	}
	return s.store.listFollowing(ctx, ownerID)
}

// CanViewProfile is canView with a bool instead of a sentinel, for the other features that
// need the answer rather than the refusal.
// ! this is the rule's one public entry point and it must stay that way. "can A see B?" gates
// ! the profile, the social graph AND the author's post list, and three copies of a three part
// ! rule is three chances for one of them to drift into leaking a private account
func (s *Service) CanViewProfile(ctx context.Context, viewerID, ownerID int64) (bool, error) {
	err := s.canView(ctx, viewerID, ownerID)
	//? both refusals collapse to a plain no, and only here. Followers/Following still need
	//? errUserMissing separately, because a 404 on a list route has a body to fill. a caller
	//? asking "may i look" gets a yes or a no — an account that does not exist is a no, and
	//? the api already treats "gone" and "not yours" as indistinguishable on purpose
	if errors.Is(err, errNoRequest) || errors.Is(err, errUserMissing) {
		return false, nil
	}
	//! anything else is a real failure — a dead connection, a locked table — and has to
	//! travel. swallowing it here would turn a database outage into "this profile is private"
	if err != nil {
		return false, err
	}
	return true, nil
}

// canView returns nil when the viewer may see owner's social graph, it reuses the same
// three part rule the profile uses, a missing owner surfaces as errUserMissing
func (s *Service) canView(ctx context.Context, viewerID, ownerID int64) error {
	if viewerID == ownerID {
		return nil
	}
	isPublic, err := s.store.targetState(ctx, ownerID)
	if err != nil {
		return err
	}
	if isPublic {
		return nil
	}
	ok, err := s.store.isAcceptedFollower(ctx, viewerID, ownerID)
	if err != nil {
		return err
	}
	if !ok {
		return errNoRequest
	}
	return nil
}

// IsConnected is for other features that need the follow relation without caring how its
// stored. chat calls it through its own FollowChecker interface, so chat never imports us
// ! note: this only answers the question, it doesnt decide anything with the answer.
// ! "two unconnected users cant chat" is chat's rule and lives in chat's service, otherwise
// ! we'd end up with follow rules spread over two modules
func (s *Service) IsConnected(ctx context.Context, a, b int64) (bool, error) {
	return s.store.connected(ctx, a, b)
}
