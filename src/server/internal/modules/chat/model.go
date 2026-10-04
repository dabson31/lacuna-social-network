package chat

import "time"

type DirectMessages struct {
	ID          int64     `json:"id"`
	SenderID    int64     `json:"sender_id"`
	RecipientID int64     `json:"recipient_id"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
	ReadAt      time.Time `json:"read_at"`
}

type GroupMessage struct {
	ID        int64     `json:"id"`
	GroupID   int64     `json:"group_id"`
	SenderID  int64     `json:"sender_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	//todo readAt will be optional since i dont know if i want to keep track of every user who read its
}

type Conversation struct {
	PeerID      int64     `json:"peer_id"`
	LastMessage string    `json:"last_message"`
	LastAt      time.Time `json:"last_at"`
	Unread      int       `json:"unread"` // number of unread messages, this will be incremented based on the user
}
