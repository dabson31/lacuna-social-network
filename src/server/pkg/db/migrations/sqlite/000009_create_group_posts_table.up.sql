-- no privacy column because if you're in the group, you're obviously going to see everything
CREATE TABLE group_posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    author_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    image_path TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);
-- sort id in descending order
CREATE INDEX idx_group_posts_group ON group_posts(group_id, id DESC);