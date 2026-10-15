package chat

import "context"

// the stuff chat needs to know but doesnt own.
//
// each one is declared here, in the consumer, and satisfied by whichever module actually
// owns the data: follows.Service, users.Service, groups.Service. app/ introduces them.
// the point of declaring them on this side is that follows never grows a method whose only
// reason to exist is to please chat
//! these are also the test seam, service_test.go fakes all three in about four lines each,
//! which is how the dm gate gets tested without a follows table anywhere near it

// FollowChecker answers whether two people have an accepted follow between them, either way
type FollowChecker interface {
	IsConnected(ctx context.Context, a, b int64) (bool, error)
}

// ProfileChecker answers the two things the dm gate needs about whoever is being messaged:
// do they exist, and is their profile public
type ProfileChecker interface {
	ProfileState(ctx context.Context, userID int64) (exists, isPublic bool, err error)
}

// GroupChecker gates group chat and hands over the fan out list
type GroupChecker interface {
	IsMember(ctx context.Context, groupID, userID int64) (bool, error)
	MemberIDs(ctx context.Context, groupID int64) ([]int64, error)
}

// Pusher is the slice of the realtime hub that chat actually uses. narrowing it to one
// method means the service can be tested without a socket, and the service physically
// cannot register, unregister or close a connection even by accident
type Pusher interface {
	SendToUsers(userIDs []int64, frame []byte)
}
