-- Revert to original language fallback logic with 'en' fallback
CREATE OR REPLACE FUNCTION get_default_user_language() RETURNS TEXT AS $$
BEGIN
    RETURN (SELECT value_json FROM config_keys WHERE key = 'defaults.userLanguage')::TEXT;
EXCEPTION WHEN OTHERS THEN
    RETURN 'en';
END;
$$ LANGUAGE plpgsql STABLE;

CREATE OR REPLACE FUNCTION trg_users_ensure_language() RETURNS TRIGGER AS $$
DECLARE
    def_lang TEXT;
BEGIN
    IF NEW.language IS NULL THEN
        def_lang := get_default_user_language();
        def_lang := trim(both '"' from def_lang);
        
        IF NOT EXISTS (SELECT 1 FROM languages WHERE code = def_lang) THEN
            def_lang := 'en';
        END IF;
        
        NEW.language := def_lang;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
