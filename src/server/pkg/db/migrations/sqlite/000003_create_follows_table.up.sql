-- both add and delete in one table
-- public accounts accept the request right away, but private accounts are sent a pending request
CREATE TABLE follows (
    id INTEGER INTEGER PRIMARY KEY AUTOINCREMENT,
    follower_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (stauts IN ('pending', 'accepted')),
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now')),
    -- this covers who does this person follow
    UNIQUE (follower_id, followee_id),
    CHECK (follower_id <> followee_id)
);
-- conversely, this checks for who follows this person instead
CREATE INDEX idx_follows_followee ON follows(followee_id, status);