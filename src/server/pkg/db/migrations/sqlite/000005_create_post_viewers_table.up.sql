CREATE TABLE post_viewers (
    post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, user_id)
);
CREATE INDEX idx_post_viewers_user ON post_viewers(user_id);
