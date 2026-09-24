CREATE TABLE groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    creator_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    --! join_policy is not a subject, entity or object, it is a feature for the public-groups
    --! and it is set to request by default
    join_policy TEXT NOT NULL DEFAULT 'request' CHECK (join_policy IN ('request', 'open')),
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_groups_creator ON groups(creator_id);