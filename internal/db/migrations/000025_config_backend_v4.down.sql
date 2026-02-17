INSERT INTO config_keys (key, description, data_type, is_public, is_secret, value_json) VALUES
('instance.url', 'Base URL for this instance (used in emails).', 'string', false, false, to_json('http://localhost:8080'::text)),
('mail.verificationbody', 'Body for email verification messages.', 'string', false, false, to_json('Hi,\n\nPlease verify your email by clicking the link below:\n{{.VerifyURL}}\n\nIf you did not create this account, you can ignore this email.'::text))
ON CONFLICT (key) DO UPDATE SET
	description = EXCLUDED.description,
	data_type = EXCLUDED.data_type,
	is_public = EXCLUDED.is_public,
	is_secret = EXCLUDED.is_secret,
	value_json = EXCLUDED.value_json;
