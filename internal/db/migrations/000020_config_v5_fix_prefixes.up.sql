-- Fix prefixes for keys introduced in migration 19 (config_v5)
-- Strategy:
-- 1) For each old key, create a new key with the correct prefix copying metadata/value_json
-- 2) Copy translations to the new key (upsert), then delete old translations
-- 3) Delete the old key rows
-- This avoids FK issues since config_translations.key references config_keys(key) without ON UPDATE CASCADE.

BEGIN;

-- Helper comment: mappings old -> new
-- ui.*
--   updatedSuccess               -> ui.updatedSuccess
--   deleteAccountButton          -> ui.deleteAccountButton
--   deleteAccountTitle           -> ui.deleteAccountTitle
--   deleteAccountPermanent       -> ui.deleteAccountPermanent
--   deleteAccountConfirmation    -> ui.deleteAccountConfirmation
--   confirmPasswordLabel         -> ui.confirmPasswordLabel
--   enterPasswordPlaceholder     -> ui.enterPasswordPlaceholder
--   deleteAccountAgreement       -> ui.deleteAccountAgreement
--   noUsersFound                 -> ui.noUsersFound
--   noUsersFoundDescription      -> ui.noUsersFoundDescription
--   cannotInactivateSelf         -> ui.cannotInactivateSelf
--   inactivateWarning            -> ui.inactivateWarning
-- lang.*
--   addLanguageTitle             -> lang.addLanguageTitle
--   addLanguageSubtitle          -> lang.addLanguageSubtitle
--   languageCodeLabel            -> lang.languageCodeLabel
--   displayNameLabel             -> lang.displayNameLabel
--   addingLanguage               -> lang.addingLanguage
--   addLanguageButton            -> lang.addLanguageButton

-- Functionally repeat the same block per mapping

-- A utility CTE-based pattern to perform one mapping
-- We inline per-key for clarity and reliability in simple SQL migrations

-- ui.updatedSuccess
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'ui.updatedSuccess', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'updatedSuccess'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'ui.updatedSuccess', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'updatedSuccess'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'updatedSuccess';
DELETE FROM config_keys WHERE key = 'updatedSuccess';

-- ui.deleteAccountButton
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'auth.deleteAccountButton', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'deleteAccountButton'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'auth.deleteAccountButton', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'deleteAccountButton'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'deleteAccountButton';
DELETE FROM config_keys WHERE key = 'deleteAccountButton';

-- ui.deleteAccountTitle
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'auth.deleteAccountTitle', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'deleteAccountTitle'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'auth.deleteAccountTitle', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'deleteAccountTitle'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'deleteAccountTitle';
DELETE FROM config_keys WHERE key = 'deleteAccountTitle';

-- ui.deleteAccountPermanent
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'auth.deleteAccountPermanent', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'deleteAccountPermanent'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'auth.deleteAccountPermanent', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'deleteAccountPermanent'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'deleteAccountPermanent';
DELETE FROM config_keys WHERE key = 'deleteAccountPermanent';

-- ui.deleteAccountConfirmation
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'auth.deleteAccountConfirmation', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'deleteAccountConfirmation'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'auth.deleteAccountConfirmation', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'deleteAccountConfirmation'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'deleteAccountConfirmation';
DELETE FROM config_keys WHERE key = 'deleteAccountConfirmation';

-- ui.confirmPasswordLabel
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'auth.confirmPasswordLabel', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'confirmPasswordLabel'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'auth.confirmPasswordLabel', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'confirmPasswordLabel'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'confirmPasswordLabel';
DELETE FROM config_keys WHERE key = 'confirmPasswordLabel';

-- ui.enterPasswordPlaceholder
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'auth.enterPasswordPlaceholder', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'enterPasswordPlaceholder'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'auth.enterPasswordPlaceholder', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'enterPasswordPlaceholder'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'enterPasswordPlaceholder';
DELETE FROM config_keys WHERE key = 'enterPasswordPlaceholder';

-- ui.deleteAccountAgreement
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'auth.deleteAccountAgreement', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'deleteAccountAgreement'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'auth.deleteAccountAgreement', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'deleteAccountAgreement'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'deleteAccountAgreement';
DELETE FROM config_keys WHERE key = 'deleteAccountAgreement';

-- ui.noUsersFound
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'ui.noUsersFound', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'noUsersFound'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'ui.noUsersFound', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'noUsersFound'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'noUsersFound';
DELETE FROM config_keys WHERE key = 'noUsersFound';

-- ui.noUsersFoundDescription
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'ui.noUsersFoundDescription', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'noUsersFoundDescription'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'ui.noUsersFoundDescription', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'noUsersFoundDescription'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'noUsersFoundDescription';
DELETE FROM config_keys WHERE key = 'noUsersFoundDescription';

-- ui.cannotInactivateSelf
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'ui.cannotInactivateSelf', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'cannotInactivateSelf'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'ui.cannotInactivateSelf', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'cannotInactivateSelf'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'cannotInactivateSelf';
DELETE FROM config_keys WHERE key = 'cannotInactivateSelf';

-- ui.inactivateWarning
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'ui.inactivateWarning', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'inactivateWarning'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'ui.inactivateWarning', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'inactivateWarning'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'inactivateWarning';
DELETE FROM config_keys WHERE key = 'inactivateWarning';

-- lang.addLanguageTitle
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'lang.addLanguageTitle', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'addLanguageTitle'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'lang.addLanguageTitle', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'addLanguageTitle'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'addLanguageTitle';
DELETE FROM config_keys WHERE key = 'addLanguageTitle';

-- lang.addLanguageSubtitle
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'lang.addLanguageSubtitle', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'addLanguageSubtitle'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'lang.addLanguageSubtitle', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'addLanguageSubtitle'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'addLanguageSubtitle';
DELETE FROM config_keys WHERE key = 'addLanguageSubtitle';

-- lang.languageCodeLabel
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'lang.languageCodeLabel', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'languageCodeLabel'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'lang.languageCodeLabel', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'languageCodeLabel'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'languageCodeLabel';
DELETE FROM config_keys WHERE key = 'languageCodeLabel';

-- lang.displayNameLabel
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'lang.displayNameLabel', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'displayNameLabel'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'lang.displayNameLabel', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'displayNameLabel'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'displayNameLabel';
DELETE FROM config_keys WHERE key = 'displayNameLabel';

-- lang.addingLanguage
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'lang.addingLanguage', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'addingLanguage'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'lang.addingLanguage', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'addingLanguage'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'addingLanguage';
DELETE FROM config_keys WHERE key = 'addingLanguage';

-- lang.addLanguageButton
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'lang.addLanguageButton', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'addLanguageButton'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'lang.addLanguageButton', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'addLanguageButton'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'addLanguageButton';
DELETE FROM config_keys WHERE key = 'addLanguageButton';

COMMIT;
