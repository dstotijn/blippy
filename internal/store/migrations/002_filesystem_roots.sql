CREATE TABLE IF NOT EXISTS filesystem_roots (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    path TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

ALTER TABLE agents ADD COLUMN enabled_filesystem_roots TEXT NOT NULL DEFAULT '[]';
