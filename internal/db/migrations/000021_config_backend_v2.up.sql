ALTER TABLE config_keys ADD COLUMN is_secret BOOLEAN NOT NULL DEFAULT false;

INSERT INTO config_keys (key, description, data_type, is_public, is_secret, value_json) VALUES
('mail.smtp.host', 'SMTP server host address', 'string', false, false, '"localhost"'),
('mail.smtp.port', 'SMTP server port', 'integer', false, false, '587'),
('mail.smtp.tls_mode', 'SMTP TLS mode: starttls|tls|none', 'string', false, false, '"starttls"'),
('mail.smtp.username', 'Username for SMTP authentication', 'string', false, false, '""'),
('mail.smtp.password', 'Password for SMTP authentication', 'string', false, true, '""')
ON CONFLICT (key) DO UPDATE SET
    description = EXCLUDED.description,
    data_type = EXCLUDED.data_type,
    is_public = EXCLUDED.is_public,
    is_secret = EXCLUDED.is_secret,
    value_json = CASE
                     WHEN config_keys.is_secret THEN config_keys.value_json
                     ELSE EXCLUDED.value_json
        END;
