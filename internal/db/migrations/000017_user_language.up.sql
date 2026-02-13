-- Add language column to users
ALTER TABLE users ADD COLUMN language TEXT REFERENCES languages(code) ON DELETE SET NULL;

-- Function to get the current default language from config
CREATE OR REPLACE FUNCTION get_default_user_language() RETURNS TEXT AS $$
BEGIN
    RETURN (SELECT value_json FROM config_keys WHERE key = 'defaults.userLanguage')::TEXT;
EXCEPTION WHEN OTHERS THEN
    RETURN 'en';
END;
$$ LANGUAGE plpgsql STABLE;

-- Trigger function to ensure language is never null
CREATE OR REPLACE FUNCTION trg_users_ensure_language() RETURNS TRIGGER AS $$
DECLARE
    def_lang TEXT;
BEGIN
    IF NEW.language IS NULL THEN
        def_lang := get_default_user_language();
        -- Strip quotes if it was stored as a JSON string like "en"
        def_lang := trim(both '"' from def_lang);
        
        -- Fallback if the config value doesn't exist in languages table
        IF NOT EXISTS (SELECT 1 FROM languages WHERE code = def_lang) THEN
            def_lang := 'en';
        END IF;
        
        NEW.language := def_lang;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger for both INSERT and UPDATE
CREATE TRIGGER users_ensure_language_tg
BEFORE INSERT OR UPDATE OF language ON users
FOR EACH ROW EXECUTE FUNCTION trg_users_ensure_language();

-- Backfill existing users
UPDATE users SET language = NULL WHERE language IS NULL;

-- Now make it NOT NULL
ALTER TABLE users ALTER COLUMN language SET NOT NULL;
