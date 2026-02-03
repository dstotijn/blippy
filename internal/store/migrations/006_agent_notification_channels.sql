-- Add enabled_notification_channels to agents
ALTER TABLE agents ADD COLUMN enabled_notification_channels TEXT NOT NULL DEFAULT '[]';
