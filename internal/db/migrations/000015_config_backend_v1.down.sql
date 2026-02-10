-- Remove seeded backend configuration defaults

delete from config_keys
where key in (
              'auth.allowSignup',
              'auth.requireEmailVerification',
              'auth.passwordMinLength',
              'mail.enabled',
              'mail.fromName',
              'mail.fromAddress',
              'defaults.userLanguage',
              'defaults.userTheme',
              'features.tags',
              'features.recurringTasks',
              'features.organizations',
              'instance.readOnly',
              'log.level'
    );
