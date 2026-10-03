CREATE TABLE sessions (
-- for reference, the id, without autoincrement, is the UUID that goes in the cookie
-- and a guessable sequential session id would be a vulnerability
    id  TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now')),
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
