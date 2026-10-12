package posts

import (
	"io"
	"time"
)

// public = everyone | almost_private = accepted followers | private = specific people selected (stored in post_viewers)
const (
	PrivacyPublic        = "public"
	PrivacyAlmostPrivate = "almost_private"
	PrivacyPrivate       = "private"
)

// includes a little bit of author information so a feed card can be drawn without a second call
type Post struct {
	ID           int64     `json:"id"`
	AuthorID     int64     `json:"author_id"`
	Content      string    `json:"content"`
	ImagePath    *string   `json:"image_path"`
	Privacy      string    `json:"privacy"`
	CreatedAt    time.Time `json:"created_at"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar *string   `json:"author_avatar"`
	CommentCount int       `json:"comment_count"`
	// how many people a private post was shared with, the design prints it as "selected · N"
	//? it is zero for a public or almost_private post because post_viewers is only written for
	//? a private one, so the number is meaningless outside that case and the ui only reads it there
	ViewerCount int `json:"viewer_count"`
}

type Comment struct {
	ID           int64     `json:"id"`
	PostID       int64     `json:"post_id"`
	AuthorID     int64     `json:"author_id"`
	Content      string    `json:"content"`
	ImagePath    *string   `json:"image_path"`
	CreatedAt    time.Time `json:"created_at"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar *string   `json:"author_avatar"`
}

// what the handler pulls out from the multipart form, ViewerIDs is only read when privacy = private
// image is nil if there is no file attached/sent
type CreatePostInput struct {
	Content   string
	Privacy   string
	ViewerIDs []int64
	Image     io.Reader
}

// CreateCommentInput is the same idea but for a comment, and yes a comment may carry an image
type CreateCommentInput struct {
	Content string
	Image   io.Reader
}
