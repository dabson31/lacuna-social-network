CREATE TABLE direct_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sender_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now')),
    read_at TIMESTAMP NULL,
    CHECK (sender_id <> recipient_id)
);

-- this is one's conversation history
CREATE INDEX idx_dm_pair ON direct_messages(sender_id, recipient_id, id DESC);
-- this is for the messages in the inbox
CREATE INDEX idx_dm_inbox ON direct_messages(recipient_id, id DESC);
--todo: in case it needs changing later
CREATE INDEX idx_dm_unread ON direct_messages(recipient_id, sender_id) WHERE read_at IS NULL;
