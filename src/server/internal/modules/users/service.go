package users

import (
	"context"
	"strings"
	"unicode/utf8"

	"social/src/server/internal/platform/httpx"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

// this is the profile from the user's pov, canView decides if the priv half of the struct
// or left null. So you can see it if its your own profile/if the profile is public/if you're an accepted follower
func (s *Service) Get(ctx context.Context, viewerID, targetID int64) (Profile, error) {
	r, err := s.store.base(ctx, targetID)
	if err != nil {
		return Profile{}, err
	}

	status, err := s.store.relationship(ctx, viewerID, targetID)
	if err != nil {
		return Profile{}, err
	}

	followers, following, err := s.store.counts(ctx, targetID)
	if err != nil {
		return Profile{}, err
	}

	isSelf := viewerID == targetID
	canView := isSelf || r.IsPublic || status == "accepted"

	p := Profile{
		ID:             r.ID,
		FirstName:      r.FirstName,
		LastName:       r.LastName,
		AvatarPath:     r.AvatarPath,
		Nickname:       r.Nickname,
		IsPublic:       r.IsPublic,
		FollowerCount:  followers,
		FollowingCount: following,
		IsSelf:         isSelf,
		FollowStatus:   status,
		CanView:        canView,
	}

	// only if a public account, your own account of if youre an accepted follower
	if canView {
		p.Email = &r.Email
		p.DateOfBirth = &r.DateOfBirth
		p.AboutMe = r.AboutMe
		p.CreatedAt = &r.CreatedAt
	}
	return p, nil
}

func (s *Service) Update(ctx context.Context, userID int64, in UpdateProfileInput) error {
	if fields := validateUpdate(in); fields.Any() {
		return fields
	}
	return s.store.update(ctx, userID, in)
}

func (s *Service) SetPrivacy(ctx context.Context, userID int64, isPublic bool) error {
	return s.store.setPrivacy(ctx, userID, isPublic)
}

// Search trims the query and clamps the page size so a caller cannot ask for everything
func (s *Service) Search(ctx context.Context, viewerID int64, q string, limit, offset int) ([]ListUser, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.store.search(ctx, viewerID, strings.TrimSpace(q), limit, offset)
}

func validateUpdate(in UpdateProfileInput) httpx.ValidationErrors {
	fields := httpx.ValidationErrors{}
	if in.FirstName != nil {
		if v := strings.TrimSpace(*in.FirstName); v == "" {
			fields.Add("first_name", "first name cannot be empty")
		} else if utf8.RuneCountInString(v) > 50 {
			fields.Add("first_name", "first name is too long")
		}
	}
	if in.LastName != nil {
		if v := strings.TrimSpace(*in.LastName); v == "" {
			fields.Add("last_name", "last name cannot be empty")
		} else if utf8.RuneCountInString(v) > 50 {
			fields.Add("last_name", "last name is too long")
		}
	}
	if in.Nickname != nil && utf8.RuneCountInString(strings.TrimSpace(*in.Nickname)) > 30 {
		fields.Add("nickname", "nickname must be at most 30 characters")
	}
	if in.AboutMe != nil && utf8.RuneCountInString(*in.AboutMe) > 500 {
		fields.Add("about_me", "about me must be at most 500 characters")
	}
	return fields
}

// ProfileState tells another feature whether a user exists and whether theyre public.
// thats everything needed to apply a visibility rule, chat calls it through its
// ProfileChecker interface
//? exists comes back as its own return value instead of as an error on purpose: the two
//? cases get different status codes later, a missing user is 404 and a private one is 403.
//? returning errNotFound here would flatten them into the same thing
func (s *Service) ProfileState(ctx context.Context, id int64) (exists, isPublic bool, err error) {
	return s.store.state(ctx, id)
}
