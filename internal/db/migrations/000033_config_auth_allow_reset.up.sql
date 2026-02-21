INSERT INTO config_keys (key, description, data_type, is_public, is_secret, value_json) VALUES
('auth.allowReset', 'Allow password resets.', 'boolean', false, false, 'true'::jsonb)
ON CONFLICT (key) DO UPDATE SET
	description = EXCLUDED.description,
	data_type = EXCLUDED.data_type,
	is_public = EXCLUDED.is_public,
	is_secret = EXCLUDED.is_secret,
	value_json = EXCLUDED.value_json;
