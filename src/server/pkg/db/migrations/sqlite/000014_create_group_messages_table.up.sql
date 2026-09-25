CREATE TABLE group_messages (
-- note that group_messages doesnt record who has read each message 
-- each member has the id of the last group message theyve read.
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    sender_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_gm_group ON group_messages(group_id, id DESC);
