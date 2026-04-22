-- +goose Up
CREATE TABLE email_verification_tokens (
    token       TEXT        PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT email_verification_tokens_expiry_future CHECK (expires_at > created_at)
);

CREATE INDEX idx_evt_user_id ON email_verification_tokens(user_id);

-- +goose Down
DROP TABLE IF EXISTS email_verification_tokens;
