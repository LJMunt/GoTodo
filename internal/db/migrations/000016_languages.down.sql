-- Remove foreign key constraint from config_translations
ALTER TABLE config_translations DROP CONSTRAINT IF EXISTS fk_config_translations_language;

DROP TABLE IF EXISTS languages;
