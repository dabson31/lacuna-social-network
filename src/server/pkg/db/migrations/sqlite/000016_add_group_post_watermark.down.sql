-- sqlite has supported DROP COLUMN since 3.35 and the driver bundles 3.45
ALTER TABLE group_members DROP COLUMN last_read_post_id;
