CREATE TABLE users (
                       id BIGSERIAL PRIMARY KEY,
                       email TEXT NOT NULL UNIQUE,
                       password_hash TEXT NOT NULL,
                       created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                       updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                       is_active BOOLEAN NOT NULL DEFAULT true
);
