CREATE TABLE posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    author_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    image_path TEXT,
    privacy TEXT NOT NULL CHECK (privacy IN ('public', 'almost_private', 'private')),
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_posts_author ON posts(author_id, id DESC);