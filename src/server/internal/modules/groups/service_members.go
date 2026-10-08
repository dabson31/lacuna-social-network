package groups

import (
	"context"

	"social/src/server/internal/platform/events"
)

// this file holds the membership state machine, every function here is one transition
// on the status column of group_members, invited and requested are the two pending
// states, member is the settled one, a declined or cancelled row is simply deleted

// Invite puts a user in the invited state, anyone already a member may invite, this is
// a subject requirement and easy to miss, the invitee has to be a real user and must not
// already have a row in this group
func (s *Service) Invite(ctx context.Context, groupID, inviterID, inviteeID int64) error {
	if _, err := s.requireMember(ctx, groupID, inviterID); err != nil {
		return err
	}
	if inviteeID == inviterID {
		return errForbidden
	}
	exists, err := s.store.userExists(ctx, inviteeID)
	if err != nil {
		return err
	}
	if !exists {
		return errNotFound
	}
	// invited_by records who sent it so the invite can show a name
	if err := s.store.addMember(ctx, groupID, inviteeID, StatusInvited, &inviterID); err != nil {
		return err
	}
	// tell the invitee, the entity is the group so the ui can take them to it
	if s.Bus != nil {
		gid := groupID
		s.Bus.Publish(ctx, events.Event{
			Kind:        events.GroupInvited,
			RecipientID: inviteeID,
			ActorID:     inviterID,
			EntityID:    &gid,
		})
	}
	return nil
}

// AcceptInvite flips the caller's own invited row to member, no such invited row means
// there is nothing to accept
func (s *Service) AcceptInvite(ctx context.Context, groupID, userID int64) error {
	return s.store.promote(ctx, groupID, userID, StatusInvited)
}

// DeclineInvite deletes the caller's own invited row
func (s *Service) DeclineInvite(ctx context.Context, groupID, userID int64) error {
	return s.store.removeMember(ctx, groupID, userID, StatusInvited)
}

// RequestJoin is how a non member asks to get in, an open group lets them in at once,
// a request group parks them in the requested state until the creator approves, the
// group has to exist and the caller must not already have a row
func (s *Service) RequestJoin(ctx context.Context, groupID, userID int64) (string, error) {
	g, err := s.store.byID(ctx, userID, groupID)
	if err != nil {
		return "", err
	}
	if g.MyStatus != "" {
		// already invited, requested, or a member, nothing to do
		return g.MyStatus, errMemberExists
	}
	status := StatusRequested
	if g.JoinPolicy == "open" {
		status = StatusMember
	}
	if err := s.store.addMember(ctx, groupID, userID, status, nil); err != nil {
		return "", err
	}

	// an open group needs no approval so nobody is notified, a request group tells the
	// creator so they can approve, the entity is the group
	if status == StatusRequested && s.Bus != nil {
		gid := groupID
		s.Bus.Publish(ctx, events.Event{
			Kind:        events.GroupJoinRequested,
			RecipientID: g.CreatorID,
			ActorID:     userID,
			EntityID:    &gid,
		})
	}

	return status, nil
}

// AcceptRequest approves a pending join request, only the creator may do this, we read
// the caller's role and refuse anyone who is not the creator
func (s *Service) AcceptRequest(ctx context.Context, groupID, callerID, targetID int64) error {
	if err := s.requireCreator(ctx, groupID, callerID); err != nil {
		return err
	}
	return s.store.promote(ctx, groupID, targetID, StatusRequested)
}

// DeclineRequest rejects a pending join request, creator only, it deletes the requested row
func (s *Service) DeclineRequest(ctx context.Context, groupID, callerID, targetID int64) error {
	if err := s.requireCreator(ctx, groupID, callerID); err != nil {
		return err
	}
	return s.store.removeMember(ctx, groupID, targetID, StatusRequested)
}

// Leave removes the caller's membership, the creator cannot leave because the group would
// be left without an owner, everyone else may walk out freely
func (s *Service) Leave(ctx context.Context, groupID, userID int64) error {
	status, role, err := s.store.membership(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if status != StatusMember {
		return errNotFound
	}
	if role == RoleCreator {
		return errForbidden
	}
	return s.store.removeMember(ctx, groupID, userID, StatusMember)
}

func (s *Service) Invitations(ctx context.Context, userID int64) ([]Invitation, error) {
	return s.store.invitationsFor(ctx, userID)
}

// Requests lists pending join requests for the creator to act on, creator only
func (s *Service) Requests(ctx context.Context, groupID, callerID int64) ([]JoinRequest, error) {
	if err := s.requireCreator(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	return s.store.requestsFor(ctx, groupID)
}

// requireCreator returns nil only when the caller is the group's creator, a plain member
// gets errForbidden, a stranger gets errNotFound so we do not reveal the group to them
func (s *Service) requireCreator(ctx context.Context, groupID, callerID int64) error {
	status, role, err := s.store.membership(ctx, groupID, callerID)
	if err != nil {
		return err
	}
	if status != StatusMember {
		return errNotFound
	}
	if role != RoleCreator {
		return errForbidden
	}
	return nil
}

// IsMember and MemberIDs are this module's answers to other features, chat calls both of
// them through its GroupChecker interface

// IsMember is the bool version of requireMember. requireMember itself cant be used across
// a module boundary because it returns errNotFound, which is an unexported sentinel that
// only this module's handler knows how to map to a status code. a bool is the smallest
// thing that can cross over without dragging our error handling with it
func (s *Service) IsMember(ctx context.Context, groupID, userID int64) (bool, error) {
	status, _, err := s.store.membership(ctx, groupID, userID)
	if err != nil {
		return false, err
	}
	return status == StatusMember, nil
}

// MemberIDs is the fan out list for a group chat message. it leaves nobody out, because the
// sender's own other tabs need the message too, otherwise sending from your phone leaves
// your laptop showing an old thread
//! memberIDs takes an "except" id so it can skip someone, which is what CreateEvent uses to
//! avoid notifying the creator of their own event. here we pass 0 instead, and 0 matches no
//! user since AUTOINCREMENT starts counting at 1
func (s *Service) MemberIDs(ctx context.Context, groupID int64) ([]int64, error) {
	return s.store.memberIDs(ctx, groupID, 0)
}
