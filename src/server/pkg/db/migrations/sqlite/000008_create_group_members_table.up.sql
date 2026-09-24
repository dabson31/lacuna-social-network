CREATE TABLE group_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    --! note: status is probbaly the most important thing in this entire project, 
    --! since its modularised to a single entity 
    status TEXT NOT NULL CHECK (status IN ('invited', 'requested', 'member'),
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('creator', 'member'),
    --todo: the inv does not get deleted if the user deletes their accounts
    invited_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
    -- last read message
    last_read_message_id INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now')),
    -- the same user can only belong to one group depending the ids of both
    UNIQUE (group_id, user_id),
);

-- checks which groups does the user belong to
CREATE INDEX idx_group_members_user ON group_members(user_id, status);
