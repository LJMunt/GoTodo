DELETE FROM config_keys WHERE key IN ('mail.smtp_host', 'mail.smtp_port', 'mail.smtp_tls_mode', 'smtp.username', 'smtp.password');

ALTER TABLE config_keys DROP COLUMN is_secret;
