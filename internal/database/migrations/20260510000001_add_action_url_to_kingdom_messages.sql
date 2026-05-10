-- +goose Up
ALTER TABLE kingdom_messages ADD COLUMN action_url TEXT;
ALTER TABLE kingdom_messages ADD COLUMN action_text TEXT;

-- +goose Down
ALTER TABLE kingdom_messages DROP COLUMN action_text;
ALTER TABLE kingdom_messages DROP COLUMN action_url;
