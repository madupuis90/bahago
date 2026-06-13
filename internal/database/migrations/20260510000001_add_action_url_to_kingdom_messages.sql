-- +goose Up
ALTER TABLE kingdom_messages ADD COLUMN action_url TEXT NOT NULL DEFAULT '';
ALTER TABLE kingdom_messages ADD COLUMN action_text TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE kingdom_messages DROP COLUMN action_text;
ALTER TABLE kingdom_messages DROP COLUMN action_url;
