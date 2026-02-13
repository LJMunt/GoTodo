-- Remove hardcoded 'en' dependency from language fallback logic

-- Update function to get the current default language from config without hardcoded 'en' fallback
CREATE OR REPLACE FUNCTION get_default_user_language() RETURNS TEXT AS $$
BEGIN
    RETURN (SELECT value_json FROM config_keys WHERE key = 'defaults.userLanguage')::TEXT;
EXCEPTION WHEN OTHERS THEN
    RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

-- Update trigger function to ensure language is never null using dynamic fallback
CREATE OR REPLACE FUNCTION trg_users_ensure_language() RETURNS TRIGGER AS $$
DECLARE
    def_lang TEXT;
BEGIN
    IF NEW.language IS NULL THEN
        def_lang := get_default_user_language();
        -- Strip quotes if it was stored as a JSON string like "en"
        def_lang := trim(both '"' from def_lang);
        
        -- Check if the config value exists in languages table
        IF def_lang IS NULL OR NOT EXISTS (SELECT 1 FROM languages WHERE code = def_lang) THEN
            -- Pick the first available language as a last resort
            SELECT code INTO def_lang FROM languages ORDER BY code ASC LIMIT 1;
        END IF;
        
        -- If we still have nothing (no languages at all), the NOT NULL constraint will catch it
        NEW.language := def_lang;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
