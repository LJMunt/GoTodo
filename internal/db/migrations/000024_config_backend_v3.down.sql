DELETE FROM config_keys WHERE key IN (
	'instance.url',
	'mail.verificationsubject',
	'mail.verificationbody'
);
