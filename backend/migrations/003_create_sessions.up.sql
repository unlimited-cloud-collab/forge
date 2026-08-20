CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    token_hash BYTEA NOT NULL,

    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT sessions_user_id_fk
        FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON DELETE CASCADE,

    CONSTRAINT sessions_token_hash_unique
        UNIQUE (token_hash),

    CONSTRAINT sessions_expiry_after_creation
        CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS sessions_user_id_idx
    ON sessions (user_id);

CREATE INDEX IF NOT EXISTS sessions_expires_at_idx
    ON sessions (expires_at);