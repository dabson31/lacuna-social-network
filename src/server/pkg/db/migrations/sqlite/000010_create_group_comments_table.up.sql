-- even though writing 
CREATE TABLE group_comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_post_id INTEGER NOT NULL REFERENCES group_posts(id) ON DELETE CASCADE,
    author_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    image_path TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_group_comments_post ON group_comments(group_post_id, id);