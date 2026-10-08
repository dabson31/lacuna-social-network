package groups

import (
	"io"
	"time"
)

// the three membership states, a row in group_members is always exactly one of these,
// invited means someone asked you in, requested means you asked to get in, member is in
const (
	StatusInvited   = "invited"
	StatusRequested = "requested"
	StatusMember    = "member"

	RoleCreator = "creator"
	RoleMember  = "member"
)

// Group is the row plus a member count and how the caller relates to it, MyStatus is
// empty when the caller has no row at all, the frontend uses it to pick join or enter
type Group struct {
	ID          int64     `json:"id"`
	CreatorID   int64     `json:"creator_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	JoinPolicy  string    `json:"join_policy"`
	CreatedAt   time.Time `json:"created_at"`
	MemberCount int       `json:"member_count"`
	MyStatus    string    `json:"my_status"`
	MyRole      string    `json:"my_role"`
}

// GroupDetail is a group plus its member list, the list is only filled for members,
// a non member gets the header and the count so they can draw a join button
type GroupDetail struct {
	Group
	Members []Member `json:"members"`
}

type Member struct {
	ID         int64   `json:"id"`
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	AvatarPath *string `json:"avatar_path"`
	Role       string  `json:"role"`
}

// Invitation is one pending invite shown to the invited user, it names the group and
// who did the inviting
type Invitation struct {
	GroupID     int64   `json:"group_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	InvitedByID *int64  `json:"invited_by_id"`
	InvitedBy   *string `json:"invited_by"`
}

// JoinRequest is one pending request shown to the creator, it names the asking user
type JoinRequest struct {
	UserID     int64   `json:"user_id"`
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	AvatarPath *string `json:"avatar_path"`
}

type GroupPost struct {
	ID           int64     `json:"id"`
	GroupID      int64     `json:"group_id"`
	AuthorID     int64     `json:"author_id"`
	Content      string    `json:"content"`
	ImagePath    *string   `json:"image_path"`
	CreatedAt    time.Time `json:"created_at"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar *string   `json:"author_avatar"`
	CommentCount int       `json:"comment_count"`
}

type GroupComment struct {
	ID           int64     `json:"id"`
	GroupPostID  int64     `json:"group_post_id"`
	AuthorID     int64     `json:"author_id"`
	Content      string    `json:"content"`
	ImagePath    *string   `json:"image_path"`
	CreatedAt    time.Time `json:"created_at"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar *string   `json:"author_avatar"`
}

// Event carries the going and not going tallies and the caller's own response so the
// page can show the counts and highlight the caller's pick in one call
type Event struct {
	ID            int64     `json:"id"`
	GroupID       int64     `json:"group_id"`
	CreatorID     int64     `json:"creator_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	EventTime     time.Time `json:"event_time"`
	CreatedAt     time.Time `json:"created_at"`
	GoingCount    int       `json:"going_count"`
	NotGoingCount int       `json:"not_going_count"`
	MyResponse    string    `json:"my_response"`
}

type CreateGroupInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	JoinPolicy  string `json:"join_policy"`
}

type InviteInput struct {
	UserID int64 `json:"user_id"`
}

type CreateEventInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	EventTime   string `json:"event_time"` // RFC 3339, for example 2026-12-25T18:00:00Z
}

type RespondInput struct {
	Response string `json:"response"` // going or not_going
}

// GroupPostInput and GroupCommentInput come off a multipart form so they can carry an image
type GroupPostInput struct {
	Content string
	Image   io.Reader
}
