-- Add description and json_schema fields to notification channels
ALTER TABLE notification_channels ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE notification_channels ADD COLUMN json_schema TEXT NOT NULL DEFAULT '';
