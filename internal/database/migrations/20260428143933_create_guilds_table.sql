-- +goose Up
CREATE TABLE guilds (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                 TEXT NOT NULL,
    slug                 TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'pending',
    founding_kingdom_ids BIGINT[] NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT guilds_name_unique UNIQUE (name),
    CONSTRAINT guilds_slug_unique UNIQUE (slug),
    CONSTRAINT guilds_status_valid CHECK (status IN ('pending', 'active'))
);

-- +goose Down
DROP TABLE guilds;
