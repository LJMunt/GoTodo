DROP TRIGGER IF EXISTS users_ensure_language_tg ON users;
DROP FUNCTION IF EXISTS trg_users_ensure_language();
DROP FUNCTION IF EXISTS get_default_user_language();
ALTER TABLE users DROP COLUMN IF EXISTS language;
