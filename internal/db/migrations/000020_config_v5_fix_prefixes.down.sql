-- Revert prefix fixes for keys introduced in migration 19 (config_v5)
-- Reverse of 000020_config_v5_fix_prefixes.up.sql

BEGIN;

-- Mapping new -> old
-- ui.updatedSuccess            -> updatedSuccess
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'updatedSuccess', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'ui.updatedSuccess'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'updatedSuccess', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'ui.updatedSuccess'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'ui.updatedSuccess';
DELETE FROM config_keys WHERE key = 'ui.updatedSuccess';

-- ui.deleteAccountButton       -> deleteAccountButton
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'deleteAccountButton', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'auth.deleteAccountButton'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'deleteAccountButton', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'auth.deleteAccountButton'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'auth.deleteAccountButton';
DELETE FROM config_keys WHERE key = 'auth.deleteAccountButton';

-- ui.deleteAccountTitle        -> deleteAccountTitle
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'deleteAccountTitle', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'auth.deleteAccountTitle'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'deleteAccountTitle', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'auth.deleteAccountTitle'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'auth.deleteAccountTitle';
DELETE FROM config_keys WHERE key = 'auth.deleteAccountTitle';

-- ui.deleteAccountPermanent    -> deleteAccountPermanent
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'deleteAccountPermanent', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'auth.deleteAccountPermanent'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'deleteAccountPermanent', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'auth.deleteAccountPermanent'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'auth.deleteAccountPermanent';
DELETE FROM config_keys WHERE key = 'auth.deleteAccountPermanent';

-- ui.deleteAccountConfirmation -> deleteAccountConfirmation
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'deleteAccountConfirmation', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'auth.deleteAccountConfirmation'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'deleteAccountConfirmation', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'auth.deleteAccountConfirmation'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'auth.deleteAccountConfirmation';
DELETE FROM config_keys WHERE key = 'auth.deleteAccountConfirmation';

-- ui.confirmPasswordLabel      -> confirmPasswordLabel
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'confirmPasswordLabel', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'auth.confirmPasswordLabel'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'confirmPasswordLabel', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'auth.confirmPasswordLabel'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'auth.confirmPasswordLabel';
DELETE FROM config_keys WHERE key = 'auth.confirmPasswordLabel';

-- ui.enterPasswordPlaceholder  -> enterPasswordPlaceholder
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'enterPasswordPlaceholder', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'auth.enterPasswordPlaceholder'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'enterPasswordPlaceholder', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'auth.enterPasswordPlaceholder'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'auth.enterPasswordPlaceholder';
DELETE FROM config_keys WHERE key = 'auth.enterPasswordPlaceholder';

-- ui.deleteAccountAgreement    -> deleteAccountAgreement
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'deleteAccountAgreement', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'auth.deleteAccountAgreement'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'deleteAccountAgreement', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'auth.deleteAccountAgreement'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'auth.deleteAccountAgreement';
DELETE FROM config_keys WHERE key = 'auth.deleteAccountAgreement';

-- ui.noUsersFound              -> noUsersFound
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'noUsersFound', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'ui.noUsersFound'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'noUsersFound', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'ui.noUsersFound'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'ui.noUsersFound';
DELETE FROM config_keys WHERE key = 'ui.noUsersFound';

-- ui.noUsersFoundDescription   -> noUsersFoundDescription
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'noUsersFoundDescription', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'ui.noUsersFoundDescription'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'noUsersFoundDescription', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'ui.noUsersFoundDescription'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'ui.noUsersFoundDescription';
DELETE FROM config_keys WHERE key = 'ui.noUsersFoundDescription';

-- ui.cannotInactivateSelf     -> cannotInactivateSelf
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'cannotInactivateSelf', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'ui.cannotInactivateSelf'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'cannotInactivateSelf', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'ui.cannotInactivateSelf'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'ui.cannotInactivateSelf';
DELETE FROM config_keys WHERE key = 'ui.cannotInactivateSelf';

-- ui.inactivateWarning         -> inactivateWarning
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'inactivateWarning', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'ui.inactivateWarning'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'inactivateWarning', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'ui.inactivateWarning'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'ui.inactivateWarning';
DELETE FROM config_keys WHERE key = 'ui.inactivateWarning';

-- lang.addLanguageTitle        -> addLanguageTitle
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'addLanguageTitle', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'lang.addLanguageTitle'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'addLanguageTitle', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'lang.addLanguageTitle'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'lang.addLanguageTitle';
DELETE FROM config_keys WHERE key = 'lang.addLanguageTitle';

-- lang.addLanguageSubtitle     -> addLanguageSubtitle
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'addLanguageSubtitle', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'lang.addLanguageSubtitle'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'addLanguageSubtitle', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'lang.addLanguageSubtitle'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'lang.addLanguageSubtitle';
DELETE FROM config_keys WHERE key = 'lang.addLanguageSubtitle';

-- lang.languageCodeLabel       -> languageCodeLabel
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'languageCodeLabel', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'lang.languageCodeLabel'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'languageCodeLabel', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'lang.languageCodeLabel'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'lang.languageCodeLabel';
DELETE FROM config_keys WHERE key = 'lang.languageCodeLabel';

-- lang.displayNameLabel        -> displayNameLabel
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'displayNameLabel', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'lang.displayNameLabel'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'displayNameLabel', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'lang.displayNameLabel'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'lang.displayNameLabel';
DELETE FROM config_keys WHERE key = 'lang.displayNameLabel';

-- lang.addingLanguage          -> addingLanguage
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'addingLanguage', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'lang.addingLanguage'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'addingLanguage', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'lang.addingLanguage'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'lang.addingLanguage';
DELETE FROM config_keys WHERE key = 'lang.addingLanguage';

-- lang.addLanguageButton       -> addLanguageButton
INSERT INTO config_keys (key, description, data_type, is_public, value_json, created_at, updated_at)
SELECT 'addLanguageButton', description, data_type, is_public, value_json, created_at, updated_at
FROM config_keys WHERE key = 'lang.addLanguageButton'
ON CONFLICT (key) DO NOTHING;
INSERT INTO config_translations (key, language_code, value, created_at, updated_at)
SELECT 'addLanguageButton', language_code, value, created_at, updated_at
FROM config_translations WHERE key = 'lang.addLanguageButton'
ON CONFLICT (key, language_code) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
DELETE FROM config_translations WHERE key = 'lang.addLanguageButton';
DELETE FROM config_keys WHERE key = 'lang.addLanguageButton';

COMMIT;
