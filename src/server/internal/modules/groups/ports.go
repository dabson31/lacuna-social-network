package groups

// Pusher is the slice of the realtime hub this module uses. one method, declared here in
// the consumer, so groups can be tested without a socket anywhere near it and the hub never
// grows a method whose only reason to exist is to please this package
// ? chat and notifications declare their own near identical interfaces rather than sharing
// ? one. a common "pusher" package would be a dependency every module has on every other
// ? module's needs, and duplicating a one method interface is the go way of avoiding that
type Pusher interface {
	SendToUsers(userIDs []int64, frame []byte)
}
