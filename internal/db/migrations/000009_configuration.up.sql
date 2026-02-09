-- Stores configuration keys and metadata
CREATE TABLE config_keys (
    key TEXT PRIMARY KEY,
    description TEXT,
    data_type TEXT NOT NULL DEFAULT 'string', -- e.g., 'string', 'boolean', 'number'
    is_public BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Stores translations for each key and language
CREATE TABLE config_translations (
    key TEXT REFERENCES config_keys(key) ON DELETE CASCADE,
    language_code TEXT NOT NULL, -- e.g., 'en', 'de-CH'
    value TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (key, language_code)
);

-- Index for faster lookups by language
CREATE INDEX idx_config_translations_lang ON config_translations(language_code);
