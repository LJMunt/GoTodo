ALTER TABLE users
DROP COLUMN totp_enabled,
DROP COLUMN totp_secret_enc,
DROP COLUMN totp_confirmed_at,
DROP COLUMN totp_last_used_step;
