CREATE TABLE comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    author_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    image_path TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_comments_post ON comments(post_id, id);