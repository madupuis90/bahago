-- +goose Up
CREATE TABLE kingdom_messages (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    from_kingdom_id BIGINT NOT NULL REFERENCES kingdoms(id),
    to_kingdom_id   BIGINT NOT NULL REFERENCES kingdoms(id),
    subject         TEXT NOT NULL,
    body            TEXT NOT NULL,
    read_at         TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT kingdom_messages_no_self_send CHECK (from_kingdom_id != to_kingdom_id)
);

CREATE INDEX ON kingdom_messages (to_kingdom_id);
CREATE INDEX ON kingdom_messages (from_kingdom_id);
-- Fast unread count and inbox listing per recipient
CREATE INDEX ON kingdom_messages (to_kingdom_id) WHERE read_at IS NULL AND deleted_at IS NULL;

-- +goose Down
DROP TABLE kingdom_messages;
