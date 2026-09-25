CREATE TABLE notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    -- user_id is the person receiving 
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN (
        'follow_request', 'group_invite', 'group_join_request', 'event_created')),
    -- the person creating the notification
    actor_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
--  entity_id is polymorphic because entity_id points at three tables depending on the row
-- its a placeholder that will change to a follow_request, group_inv or event_created depending on what you assign to it
    entity_id INTEGER NULL,
    read_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_notifications_inbox ON notifications(user_id, id DESC);
CREATE INDEX idx_notifications_unread ON notifications(user_id) WHERE read_at IS NULL;
