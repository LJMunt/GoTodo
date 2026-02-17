INSERT INTO config_keys (key, description, data_type, is_public, is_secret, value_json) VALUES
('instance.url', 'Frontend base URL used for email links.', 'string', false, false, to_json('http://localhost:5173'::text)),
('mail.verificationbody', 'HTML body for email verification messages.', 'string', false, false, to_json('Hi,<br><br>Please verify your email by clicking the link below:<br><a href="{{.VerifyURL}}">Verify your email</a><br><br>If you did not create this account, you can ignore this email.'::text))
ON CONFLICT (key) DO UPDATE SET
	description = EXCLUDED.description,
	data_type = EXCLUDED.data_type,
	is_public = EXCLUDED.is_public,
	is_secret = EXCLUDED.is_secret,
	value_json = EXCLUDED.value_json;
