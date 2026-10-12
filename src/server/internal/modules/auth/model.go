package auth

import (
	"io"
	"time"
)

// ts has everything about a user except their password hash obviously
// the pointer fields are optional ones (nil scans from NULL and encodes as null)
type User struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DateOfBirth string `json:"date_of_birth"`
	// "YYYY-MM-DD" calender date
	AvatarPath *string   `json:"avatar_path"`
	Nickname   *string   `json:"nickname"`
	AboutMe    *string   `json:"about_me"`
	IsPublic   bool      `json:"is_public"`
	CreatedAt  time.Time `json:"created_at"`
}

// this is what the handler pulls out of the multipart form
type RegisterInput struct {
	Email       string
	Password    string
	FirstName   string
	LastName    string
	DateOfBirth string
	Nickname    string
	AboutMe     string
	Avatar      io.Reader // nil when no file was sent
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
