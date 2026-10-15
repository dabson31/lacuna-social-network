package notifications

// Pusher is the slice of the realtime hub this module uses. one method, declared here in
// the consumer, so notifications can be tested without a socket anywhere near it
// ? chat declares its own near identical interface instead of sharing this one on purpose.
// ? a shared "pusher" package would be a dependency both modules have on each other's needs,
// ? and duplicating a one method interface is the go way of avoiding that
type Pusher interface {
	SendToUser(userID int64, frame []byte)
}
