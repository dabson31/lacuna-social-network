package realtime

import (
	"log/slog"
	"sync"
)

// Hub is the connection registry. It maps a user id to every socket that user currently
// has open, and moves bytes to them. That is all. It does not know what a message, a
// group or a notification is, which means nothing in this file changes when a feature
// is added — the feature just hands it bytes and a list of user ids.
//
// conns is a map of maps, not map[int64]*Conn, on purpose: the audit logs the same person
// in on two browsers and expects both tabs to receive. One socket per user would silently
// drop the second tab, and "silently" is the part that makes it expensive to find.
type Hub struct {
	mu    sync.RWMutex
	conns map[int64]map[*Conn]struct{}
	log   *slog.Logger
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{conns: make(map[int64]map[*Conn]struct{}), log: log}
}

// Register adds a socket. Write lock, not read lock: we mutate the map, and a concurrent
// SendToUser ranging over it at the same moment is a data race that takes the process
// down — not an error you get to handle, a crash.
func (h *Hub) Register(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[c.userID] == nil {
		h.conns[c.userID] = make(map[*Conn]struct{})
	}
	h.conns[c.userID][c] = struct{}{}
	h.log.Debug("ws register", "user", c.userID, "sockets", len(h.conns[c.userID]))
}

// Unregister removes a socket, and drops the user's bucket once it is empty so a server
// that has been up for a week does not hold an empty map for everyone who ever logged in.
//
// The read pump defers this, so however the connection dies — tab closed, wifi dropped,
// we hung up on it — a dead entry can never be left behind for a later broadcast to
// write into.
func (h *Hub) Unregister(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sockets, ok := h.conns[c.userID]
	if !ok {
		return
	}
	delete(sockets, c)
	if len(sockets) == 0 {
		delete(h.conns, c.userID)
	}
	h.log.Debug("ws unregister", "user", c.userID, "remaining", len(sockets))
}

// SendToUser queues one frame on every socket that user has open.
//
// It never blocks. Queueing is a non-blocking send inside Conn.enqueue, so one client on
// a train tunnel cannot freeze the hub and with it delivery for everybody else.
//
// The sockets are copied out under the read lock and written to outside it: enqueue may
// decide to close a hopeless connection, and anything that closes must be free to take
// the write lock without us still holding the read lock. A RWMutex is not reentrant, so
// that would be a deadlock rather than a slowdown.
func (h *Hub) SendToUser(userID int64, frame []byte) {
	h.mu.RLock()
	sockets := make([]*Conn, 0, len(h.conns[userID]))
	for c := range h.conns[userID] {
		sockets = append(sockets, c)
	}
	h.mu.RUnlock()

	for _, c := range sockets {
		c.enqueue(frame)
	}
}

// SendToUsers is the fan-out: one encoded frame, many recipients. The caller encodes once
// and we reuse the same bytes, which is also why frames must be treated as read-only once
// handed over.
func (h *Hub) SendToUsers(userIDs []int64, frame []byte) {
	for _, id := range userIDs {
		h.SendToUser(id, frame)
	}
}

// IsOnline is what a presence feature would read. Nothing uses it yet; it costs one method.
func (h *Hub) IsOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns[userID]) > 0
}

// CloseAll drops every socket. http.Server.Shutdown deliberately does not touch hijacked
// connections, so without this the browsers find out the server went away only when the
// TCP connection resets.
func (h *Hub) CloseAll() {
	h.mu.Lock()
	all := make([]*Conn, 0, len(h.conns))
	for _, sockets := range h.conns {
		for c := range sockets {
			all = append(all, c)
		}
	}
	h.conns = make(map[int64]map[*Conn]struct{})
	h.mu.Unlock()

	for _, c := range all {
		c.close()
	}
}
