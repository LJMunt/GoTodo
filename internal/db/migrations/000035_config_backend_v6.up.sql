INSERT INTO config_keys (key, description, data_type, is_public, is_secret, value_json) VALUES
('auth.allowTOTP', 'Whether or not to allow users to use TOTP for MFA.', 'boolean', true, false, to_json(false))
ON CONFLICT (key) DO UPDATE SET
	description = EXCLUDED.description,
	data_type = EXCLUDED.data_type,
	is_public = EXCLUDED.is_public,
	is_secret = EXCLUDED.is_secret,
	value_json = EXCLUDED.value_json;
