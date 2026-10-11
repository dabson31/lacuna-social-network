-- unread group posts, counted the same way unread group messages already are.
--! a watermark and not a row per member per post. the same tradeoff group_messages made in
--! 000008 and for the same reason: a group of twenty with fifty posts would otherwise be a
--! thousand rows that only ever answer one question, "how many have i not seen"
--? it also means there is no "who has read this post" to render, which nothing asks for
ALTER TABLE group_members ADD COLUMN last_read_post_id INTEGER NOT NULL DEFAULT 0;
