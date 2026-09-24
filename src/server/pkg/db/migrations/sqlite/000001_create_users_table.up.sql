CREATE TABLE users (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
--  collate nocase is for case-insensitive operations
 email  TEXT NOT NULL UNIQUE COLLATE NOCASE,
 password_hash TEXT NOT NULL,
 first_name TEXT NOT NULL,
 last_name TEXT NOT NULL,
 date_of_birth TEXT NOT NULL,
 avatar_path TEXT,
 nickname TEXT UNIQUE,
 about_me TEXT,
--  sqlite has no boolean type so this is the idiomatic way of checking true and false
 is_public INTEGER NOT NULL DEFAULT 0 CHECK (is_public IN (0, 1),
 created_at TIMESTAMP NOT NULL DEFAULT (datetime('now')
);
