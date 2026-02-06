-- name: CreateAgent :one
INSERT INTO agents (id, name, description, system_prompt, enabled_tools, enabled_notification_channels, model, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAgent :one
SELECT * FROM agents WHERE id = ?;

-- name: ListAgents :many
SELECT * FROM agents ORDER BY created_at DESC;

-- name: UpdateAgent :one
UPDATE agents
SET name = ?, description = ?, system_prompt = ?, enabled_tools = ?, enabled_notification_channels = ?, model = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE id = ?;

-- name: CreateConversation :one
INSERT INTO conversations (id, agent_id, title, previous_response_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetConversation :one
SELECT * FROM conversations WHERE id = ?;

-- name: ListConversations :many
SELECT * FROM conversations WHERE agent_id = ? ORDER BY updated_at DESC;

-- name: ListAllConversations :many
SELECT * FROM conversations ORDER BY updated_at DESC;

-- name: UpdateConversation :one
UPDATE conversations
SET title = ?, previous_response_id = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteConversation :exec
DELETE FROM conversations WHERE id = ?;

-- name: CreateMessage :one
INSERT INTO messages (id, conversation_id, role, content, tool_executions, created_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetMessagesByConversation :many
SELECT * FROM messages WHERE conversation_id = ? ORDER BY created_at ASC;

-- Triggers

-- name: CreateTrigger :one
INSERT INTO triggers (id, agent_id, name, prompt, cron_expr, enabled, next_run_at, model, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTrigger :one
SELECT * FROM triggers WHERE id = ?;

-- name: ListTriggersByAgent :many
SELECT * FROM triggers WHERE agent_id = ? ORDER BY created_at DESC;

-- name: ListAllTriggers :many
SELECT * FROM triggers ORDER BY created_at DESC;

-- name: UpdateTrigger :one
UPDATE triggers SET name = ?, prompt = ?, cron_expr = ?, enabled = ?, next_run_at = ?, updated_at = ?
WHERE id = ? RETURNING *;

-- name: DeleteTrigger :exec
DELETE FROM triggers WHERE id = ?;

-- name: GetDueTriggers :many
SELECT * FROM triggers WHERE enabled = 1 AND next_run_at <= ? ORDER BY next_run_at ASC;

-- name: UpdateTriggerNextRun :exec
UPDATE triggers SET next_run_at = ?, updated_at = ? WHERE id = ?;

-- Trigger Runs

-- name: CreateTriggerRun :one
INSERT INTO trigger_runs (id, trigger_id, conversation_id, status, error_message, started_at, finished_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTriggerRun :exec
UPDATE trigger_runs SET status = ?, error_message = ?, conversation_id = ?, finished_at = ?
WHERE id = ?;

-- name: ListTriggerRuns :many
SELECT * FROM trigger_runs WHERE trigger_id = ? ORDER BY started_at DESC LIMIT ?;

-- Notification Channels

-- name: CreateNotificationChannel :one
INSERT INTO notification_channels (id, name, type, config, description, json_schema, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetNotificationChannelByName :one
SELECT * FROM notification_channels WHERE name = ?;

-- name: GetNotificationChannel :one
SELECT * FROM notification_channels WHERE id = ?;

-- name: ListNotificationChannels :many
SELECT * FROM notification_channels ORDER BY created_at DESC;

-- name: UpdateNotificationChannel :one
UPDATE notification_channels SET name = ?, type = ?, config = ?, description = ?, json_schema = ?, updated_at = ?
WHERE id = ? RETURNING *;

-- name: DeleteNotificationChannel :exec
DELETE FROM notification_channels WHERE id = ?;
