CREATE TABLE event_responses (
    event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    response TEXT NOT NULL CHECK (response IN ('going', 'not_going')),
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (event_id, user_id)
);