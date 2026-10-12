package follows

import "time"

// RequestItem is the incoming follow request shown to the person being followed
// FollowID is = follows row id, it is accept and decline are based on
type RequestItem struct {
	FollowID   int64     `json:"follow_id"`
	ActorID    int64     `json:"actor_id"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	AvatarPath *string   `json:"avatar_path"`
	CreatedAt  time.Time `json:"created_at"`
}

// UserCard is one entry in a followers or following list
type UserCard struct {
	ID         int64   `json:"id"`
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	Nickname   *string `json:"nickname"`
	AvatarPath *string `json:"avatar_path"`
	IsPublic   bool    `json:"is_public"`
}

// FollowResult tells the caller what happened
type FollowResult struct {
	Status string `json:"status"` // pending or accepted
}
