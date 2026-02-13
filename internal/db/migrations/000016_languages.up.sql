CREATE TABLE languages (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Insert default languages to avoid foreign key violations
INSERT INTO languages (code, name) VALUES
('en', 'English'),
('fr', 'French'),
('de', 'German')
ON CONFLICT (code) DO NOTHING;

-- Add foreign key constraint to config_translations
ALTER TABLE config_translations
ADD CONSTRAINT fk_config_translations_language
FOREIGN KEY (language_code) REFERENCES languages(code) ON DELETE CASCADE;
