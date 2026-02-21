INSERT INTO config_keys (key, description, data_type, is_public, is_secret, value_json) VALUES
('mail.reset_password_subject', 'Subject for password reset e-mails.', 'string', false, false, to_json('Reset your password'::text)),
('mail.reset_password_body', 'HTML body for password reset e-mails.', 'string', false, false, to_json('Hi,<br><br>You requested a password reset. Please click the link below to set a new password:<br><a href="{{.ResetURL}}">Reset password</a><br><br>If you did not request this, you can safely ignore this email.'::text))
ON CONFLICT (key) DO UPDATE SET
	description = EXCLUDED.description,
	data_type = EXCLUDED.data_type,
	is_public = EXCLUDED.is_public,
	is_secret = EXCLUDED.is_secret,
	value_json = EXCLUDED.value_json;
