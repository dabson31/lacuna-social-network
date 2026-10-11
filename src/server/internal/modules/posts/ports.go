package posts

import "context"

// follows.Service, which owns "can A see B?". app/ introduces the two, i.e., the same arrangement chat/ports.go uses for its three
// ! this is a profile rule, not a post rule:
// ! `visible` predicate in store.go decides whether one post may be read, per post
// this  decides whether an author's record may be browsed at all, which is what makes a private
// account's profile private: without it a stranger reads "this record is private" and then
// the owner's public posts listed directly underneath it
type ProfileVisibility interface {
	CanViewProfile(ctx context.Context, viewerID, ownerID int64) (bool, error)
}
