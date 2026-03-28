-- +goose Up
CREATE TABLE email_verifications (
    token       TEXT        PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ev_user_id ON email_verifications(user_id);

-- +goose Down
DROP TABLE IF EXISTS email_verifications;
