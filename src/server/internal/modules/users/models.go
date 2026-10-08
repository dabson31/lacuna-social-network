package users

import "time"

// this is what GET users id returns vvv, pointer fields at the bottom are null, a non follower
// looking at a priv account only gets the public header
type Profile struct {
	ID         int64   `json:"id"`
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	AvatarPath *string `json:"avatar_path"`
	Nickname   *string `json:"nickname"`
	IsPublic   bool    `json:"is_public"`

	FollowerCount  int `json:"follower_count"`
	FollowingCount int `json:"following_count"`

	// how the viewer relates to this profile, the frontend uses these to pick the button
	IsSelf       bool   `json:"is_self"`
	FollowStatus string `json:"follow_status"` // three statuses: empty, pending, accepted
	CanView      bool   `json:"can_view"`

	// these are only filled when CanView is true, else stays null
	Email       *string    `json:"email"`
	DateOfBirth *string    `json:"date_of_birth"`
	AboutMe     *string    `json:"about_me"`
	CreatedAt   *time.Time `json:"created_at"`
}

// plain users row before visibility rules applied
type raw struct {
	ID          int64
	FirstName   string
	LastName    string
	AvatarPath  *string
	Nickname    *string
	AboutMe     *string
	Email       string
	DateOfBirth string
	IsPublic    bool
	CreatedAt   time.Time
}

// pointers let us tell a field the client omitted from one it deliberately cleared
// only changable fields, pointers let us know the field
type UpdateProfileInput struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Nickname  *string `json:"nickname"`
	AboutMe   *string `json:"about_me"`
}

// public <=> private toggle
type PrivacyInput struct {
	IsPublic bool `json:"is_public"`
}

// row of the browse and search results (small intentionally)
type ListUser struct {
	ID         int64   `json:"id"`
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	Nickname   *string `json:"nickname"`
	AvatarPath *string `json:"avatar_path"`
	IsPublic   bool    `json:"is_public"`
}
