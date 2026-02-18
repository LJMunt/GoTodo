CREATE TABLE IF NOT EXISTS jwt_revocations (
    jti TEXT PRIMARY KEY CHECK (length(jti) = 64),
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    reason TEXT
);

CREATE INDEX IF NOT EXISTS idx_jwt_revocations_expires_at ON jwt_revocations(expires_at);
CREATE INDEX IF NOT EXISTS idx_jwt_revocations_user_id ON jwt_revocations(user_id);
