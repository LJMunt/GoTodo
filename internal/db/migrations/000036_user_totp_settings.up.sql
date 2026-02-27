ALTER TABLE users
ADD COLUMN totp_enabled BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN totp_secret_enc TEXT,
ADD COLUMN totp_confirmed_at TIMESTAMPTZ,
ADD COLUMN totp_last_used_step BIGINT;
