package chat

import "time"

// the frame types this module owns. they live here and not in platform/realtime because
// realtime is not supposed to know what a "dm" is, thats the whole thing keeping it a
// transport instead of a feature
// ? these are mirrored in src/shared/constants/ws.ts. if one side changes and the other
// ? doesnt, nothing errors, the frontend just quietly stops reacting to a frame type
const (
	FrameDMMessage    = "dm.message"
	FrameDMTyping     = "dm.typing"
	FrameGroupMessage = "group.message"
)

// DirectMessage is one direct_messages row joined with whoever sent it, so a frame pushed
// over the socket and a row fetched from history are the exact same shape. the frontend
// then needs one renderer instead of two
// ! ReadAt is a pointer because the column is NULL until the recipient opens the chat.
// ! scanning a NULL into a plain time.Time blows up at runtime with
// ! "converting NULL to time.Time is unsupported", so every nullable column below is a pointer
type DirectMessage struct {
	ID           int64      `json:"id"`
	SenderID     int64      `json:"sender_id"`
	RecipientID  int64      `json:"recipient_id"`
	Content      string     `json:"content"`
	CreatedAt    time.Time  `json:"created_at"`
	ReadAt       *time.Time `json:"read_at"`
	SenderName   string     `json:"sender_name"`
	SenderAvatar *string    `json:"sender_avatar"`
}

// GroupMessage has no read state and thats on purpose. tracking who read what in a group
// would need a row per member per message, so group_members.last_read_message_id holds a
// watermark instead: one number per member, and enough to count whats unread
type GroupMessage struct {
	ID           int64     `json:"id"`
	GroupID      int64     `json:"group_id"`
	SenderID     int64     `json:"sender_id"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
	SenderName   string    `json:"sender_name"`
	SenderAvatar *string   `json:"sender_avatar"`
}

// Conversation is one row of the dm list: who, what was said last, and how much of it you
// havent read. LastMessage is empty and LastAt is nil for someone you can chat with but
// never have, which is intentional, the list doubles as "who can i even start a chat with"
type Conversation struct {
	PeerID      int64      `json:"peer_id"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	AvatarPath  *string    `json:"avatar_path"`
	LastMessage string     `json:"last_message"`
	LastAt      *time.Time `json:"last_at"`
	Unread      int        `json:"unread"`
}

// GroupConversation is the same row but for a group chat
type GroupConversation struct {
	GroupID     int64      `json:"group_id"`
	Title       string     `json:"title"`
	LastMessage string     `json:"last_message"`
	LastSender  string     `json:"last_sender"`
	LastAt      *time.Time `json:"last_at"`
	Unread      int        `json:"unread"`
}

// Inbox is what GET /api/conversations gives back. dms and groups come together because
// the ui draws them in one sidebar, and two requests to fill one list is a waterfall for
// no reason
type Inbox struct {
	Direct []Conversation      `json:"direct"`
	Groups []GroupConversation `json:"groups"`
}

type SendDMInput struct {
	RecipientID int64  `json:"recipient_id"`
	Content     string `json:"content"`
}

type SendGroupMessageInput struct {
	Content string `json:"content"`
}

// TypingIn and TypingOut are two types for what looks like one message, deliberately.
// the field means a different thing each way: "who im typing to" coming in, "who is typing
// at you" going out. one struct with a dual meaning field is how you end up echoing a
// typing notice back at the person who sent it
type TypingIn struct {
	PeerID int64 `json:"peer_id"`
}

type TypingOut struct {
	FromUserID int64 `json:"from_user_id"`
}
