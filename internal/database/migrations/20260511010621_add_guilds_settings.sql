-- +goose Up
ALTER TABLE guilds
    ADD COLUMN settings JSONB NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE guilds
    DROP COLUMN settings;
