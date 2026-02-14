CREATE TABLE IF NOT EXISTS agent_files (
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (agent_id, path)
);
