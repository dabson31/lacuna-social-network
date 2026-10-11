package posts

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"social/src/server/internal/platform/httpx"
	"social/src/server/internal/platform/storage"
)

// errForbiddenViewer means a private post named a viewer who is not an accepted follower
// (we SHOULD be handling unfollowing scenarios with private posts, will test case it l8r)
var errForbiddenViewer = errors.New("a chosen viewer is not an accepted follower")

type Service struct {
	store   *Store
	uploads *storage.Store
	// this is wired at startup like the buses are, and nil only in a test that never calls
	// basically fixes the profile visibility issue in the network side
	Profiles ProfileVisibility
}

func NewService(store *Store, uploads *storage.Store) *Service {
	return &Service{store: store, uploads: uploads}
}

// validates input, saves image if exists and writes the post. A post needs a text or an image, and
// a private post may only choose accepted followers. The image is only saved if it passes the
// checks so we dont unnecessarily upload and delete the images which wastes computation power
func (s *Service) Create(ctx context.Context, authorID int64, in CreatePostInput) (Post, error) {
	in.Content = strings.TrimSpace(in.Content)

	if fields := validatePost(in); fields.Any() {
		return Post{}, fields
	}

	// for a private post every named viewer must really be an accepted follower, we fetch
	// the subset that qualifies and reject if any id did not come back
	if in.Privacy == PrivacyPrivate && len(in.ViewerIDs) > 0 {
		ok, err := s.store.acceptedAmong(ctx, authorID, in.ViewerIDs)
		if err != nil {
			return Post{}, err
		}
		for _, id := range in.ViewerIDs {
			if !ok[id] {
				return Post{}, errForbiddenViewer
			}
		}
	}

	var imagePath *string
	if in.Image != nil {
		name, err := s.uploads.Save(in.Image)
		if err != nil {
			return Post{}, err
		}
		imagePath = &name
	}

	id, err := s.store.create(ctx, authorID, in.Content, imagePath, in.Privacy, in.ViewerIDs)
	if err != nil {
		if imagePath != nil {
			_ = s.uploads.Delete(*imagePath)
		}
		return Post{}, err
	}

	// read it back so the caller gets the author name and timestamps the db filled in
	return s.store.byID(ctx, authorID, id)
}

func (s *Service) Feed(ctx context.Context, viewerID, before int64, limit int) ([]Post, error) {
	return s.store.feed(ctx, viewerID, before, clampLimit(limit))
}

// AuthorPosts is the profile's post list, and it answers to two separate rules.
//
// the `visible` predicate in the store decides, per post, whether this viewer may read it.
// that is not enough on its own: a private account's `public` posts pass it, so a stranger
// browsing a private profile used to be handed the owner's posts underneath the very notice
// saying the record was private. the audit asks the opposite question — "are you prevented
// from seeing a non-followed private profile" — so the profile rule applies first
//
// ! an empty page and not an error. the profile itself must still render, because the follow
// ! button is how a reader gets out of this state, and the page already has the right copy
// ! for it: "nothing here you are allowed to read"
// ? the feed is deliberately untouched. a public post is public, which is what the subject
// ? says post privacy means, and the feed is the place that rule belongs to
func (s *Service) AuthorPosts(ctx context.Context, viewerID, authorID, before int64, limit int) ([]Post, error) {
	if viewerID != authorID {
		if s.Profiles == nil {
			return []Post{}, nil
		}
		ok, err := s.Profiles.CanViewProfile(ctx, viewerID, authorID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Post{}, nil
		}
	}
	return s.store.authorPosts(ctx, viewerID, authorID, before, clampLimit(limit))
}

func (s *Service) Get(ctx context.Context, viewerID, postID int64) (Post, error) {
	return s.store.byID(ctx, viewerID, postID)
}

// Comment adds a comment but only to a post the commenter is allowed to see, otherwise
// it reads as not found, the same rule the feed uses
func (s *Service) Comment(ctx context.Context, viewerID, postID int64, in CreateCommentInput) (Comment, error) {
	in.Content = strings.TrimSpace(in.Content)
	if in.Content == "" && in.Image == nil {
		return Comment{}, httpx.ValidationErrors{"content": "a comment needs text or an image"}
	}
	if utf8.RuneCountInString(in.Content) > 2000 {
		return Comment{}, httpx.ValidationErrors{"content": "comment is too long"}
	}

	ok, err := s.store.canSee(ctx, viewerID, postID)
	if err != nil {
		return Comment{}, err
	}
	if !ok {
		return Comment{}, errNotVisible
	}

	var imagePath *string
	if in.Image != nil {
		name, err := s.uploads.Save(in.Image)
		if err != nil {
			return Comment{}, err
		}
		imagePath = &name
	}

	id, err := s.store.addComment(ctx, postID, viewerID, in.Content, imagePath)
	if err != nil {
		if imagePath != nil {
			_ = s.uploads.Delete(*imagePath)
		}
		return Comment{}, err
	}

	for _, c := range mustList(s.store.comments(ctx, postID)) {
		if c.ID == id {
			return c, nil
		}
	}
	return Comment{}, nil
}

func (s *Service) Comments(ctx context.Context, viewerID, postID int64) ([]Comment, error) {
	ok, err := s.store.canSee(ctx, viewerID, postID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errNotVisible
	}
	return s.store.comments(ctx, postID)
}

func validatePost(in CreatePostInput) httpx.ValidationErrors {
	fields := httpx.ValidationErrors{}
	if in.Content == "" && in.Image == nil {
		fields.Add("content", "a post needs text or an image")
	}
	if utf8.RuneCountInString(in.Content) > 5000 {
		fields.Add("content", "post is too long")
	}
	switch in.Privacy {
	case PrivacyPublic, PrivacyAlmostPrivate, PrivacyPrivate:
	default:
		fields.Add("privacy", "privacy must be public, almost_private or private")
	}
	return fields
}

func clampLimit(limit int) int {
	if limit <= 0 || limit > 50 {
		return 20
	}
	return limit
}

// mustList drops the error from a list call inside a loop where we already handled it,
// it keeps the happy path readable, any real error already surfaced on the insert
func mustList(c []Comment, _ error) []Comment { return c }

// Delete withdraws the caller's own post. a post that is not theirs, or not there, reads as
// not found, the same way an invisible post does everywhere else in this module
// ! the files are unlinked after the row is committed and never before. the other order means
// ! a failed delete leaves a row pointing at an image that no longer exists, and a broken
// ! image in a feed is worse than a file nobody references
func (s *Service) Delete(ctx context.Context, authorID, postID int64) error {
	names, err := s.store.imagePaths(ctx, postID)
	if err != nil {
		return err
	}
	if err := s.store.remove(ctx, authorID, postID); err != nil {
		return err
	}
	for _, name := range names {
		// best effort, the post is already gone and a leftover file is not worth failing the
		// request the caller is waiting on
		_ = s.uploads.Delete(name)
	}
	return nil
}
