package auth

// Sockets is the slice of the realtime hub this module uses: one method, declared here in
// the consumer, so auth can be tested without a socket anywhere near it and the hub never
// grows a method to please this package.
//
// ! it takes the session id and not only the user id, and that is the entire contract. a
// ! logout ends ONE login. the same person may be signed in from another browser on a
// ! session of its own, and hanging up on that one too would break something the audit
// ! checks: two browsers logged in independently stay independent.
// ? the same shape as notifications.Pusher and chat.Pusher, declared separately for the same
// ? stated reason — a shared "sockets" package would be a dependency every module has on
// ? every other module's needs
type Sockets interface {
	CloseSession(userID int64, sessionID string)
}
