-- nothing to undo. the up is a one-off DELETE of rows that pointed at follows rows which no
-- longer exist, and deleted data cannot be reconstructed from what is left in the schema.
--! it is written out rather than left missing because every up in this folder has a down,
--! and a migration with no file reads as an oversight instead of a decision. golang-migrate
--! needs the file to exist, not to do anything
SELECT 1;
