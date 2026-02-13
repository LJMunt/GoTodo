-- Seed backend configuration defaults

insert into config_keys (key, description, data_type, is_public, value_json) values
                                                                                 ('auth.allowSignup',
                                                                                  'Allow new users to sign up.',
                                                                                  'boolean',
                                                                                  false,
                                                                                  'true'),

                                                                                 ('auth.requireEmailVerification',
                                                                                  'Require email verification on signup.',
                                                                                  'boolean',
                                                                                  false,
                                                                                  'false'),

                                                                                 ('auth.passwordMinLength',
                                                                                  'Minimum password length.',
                                                                                  'number',
                                                                                  false,
                                                                                  '8'),

                                                                                 ('mail.enabled',
                                                                                  'Enable outbound email sending.',
                                                                                  'boolean',
                                                                                  false,
                                                                                  'false'),

                                                                                 ('mail.fromName',
                                                                                  'From name used in outbound emails.',
                                                                                  'string',
                                                                                  false,
                                                                                  '"Todexia"'),

                                                                                 ('mail.fromAddress',
                                                                                  'From address used in outbound emails.',
                                                                                  'string',
                                                                                  false,
                                                                                  '"support@todexia.app"'),

                                                                                 ('defaults.userLanguage',
                                                                                  'Default language for new users (BCP-47 tag).',
                                                                                  'string',
                                                                                  false,
                                                                                  '"en"'),

                                                                                 ('defaults.userTheme',
                                                                                  'Default theme for new users: system|light|dark.',
                                                                                  'string',
                                                                                  false,
                                                                                  '"system"'),

                                                                                 ('features.tags',
                                                                                  'Enable tags feature.',
                                                                                  'boolean',
                                                                                  false,
                                                                                  'true'),

                                                                                 ('features.recurringTasks',
                                                                                  'Enable recurring tasks.',
                                                                                  'boolean',
                                                                                  false,
                                                                                  'true'),

                                                                                 ('features.organizations',
                                                                                  'Enable organizations feature.',
                                                                                  'boolean',
                                                                                  false,
                                                                                  'false'),

                                                                                 ('instance.readOnly',
                                                                                  'Reject write operations (maintenance mode).',
                                                                                  'boolean',
                                                                                  false,
                                                                                  'false'),

                                                                                 ('log.level',
                                                                                  'Logging level: debug|info|warn|error.',
                                                                                  'string',
                                                                                  false,
                                                                                  '"error"')

on conflict (key) do update
    set description = excluded.description,
        data_type   = excluded.data_type,
        is_public   = excluded.is_public,
        value_json  = excluded.value_json;
