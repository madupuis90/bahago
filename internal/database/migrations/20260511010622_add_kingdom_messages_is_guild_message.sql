-- +goose Up
ALTER TABLE kingdom_messages
    ADD COLUMN is_guild_message BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE kingdom_messages
    DROP COLUMN is_guild_message;
