insert into config_keys (key, description, data_type, is_public) values
  ('branding.appName', 'Application name', 'string', true),
  ('branding.appLogoInitial', 'Logo initial letter', 'string', true),
  ('branding.supportEmail', 'Support email', 'string', true),

  ('auth.loginTitle', 'Login page title', 'string', true),
  ('auth.loginSubtitle', 'Login page subtitle', 'string', true),
  ('auth.signupTitle', 'Signup page title', 'string', true),
  ('auth.signupSubtitle', 'Signup page subtitle', 'string', true),

  ('ui.agendaTitle', 'Agenda page title', 'string', true),
  ('ui.agendaEmptyStateTitle', 'Agenda empty title', 'string', true),
  ('ui.agendaEmptyStateText', 'Agenda empty text', 'string', true),

  ('navigation.agenda', 'Nav label: agenda', 'string', true),
  ('navigation.projects', 'Nav label: projects', 'string', true),
  ('navigation.dashboard', 'Nav label: dashboard', 'string', true),
  ('navigation.users', 'Nav label: users', 'string', true),
  ('navigation.configuration', 'Nav label: configuration', 'string', true),
  ('navigation.jobs', 'Nav label: jobs', 'string', true),
  ('navigation.userSettings', 'Nav label: user settings', 'string', true),
  ('navigation.administration', 'Nav label: administration', 'string', true),
  ('navigation.support', 'Nav label: support', 'string', true),
  ('navigation.logout', 'Log out', 'string', true)
on conflict (key) do nothing;

-- seed en translations
insert into config_translations (key, language_code, value)
values
  ('branding.appName', 'en', 'Gotodo'),
  ('branding.appLogoInitial', 'en', 'G'),
  ('branding.supportEmail', 'en', 'support@todexia.app'),

  ('auth.loginTitle', 'en', 'Welcome back'),
  ('auth.loginSubtitle', 'en', 'Step back into your agenda.'),
  ('auth.signupTitle', 'en', 'Create account'),
  ('auth.signupSubtitle', 'en', 'Spin up a workspace that belongs to you.'),

  ('ui.agendaTitle', 'en', 'Your Agenda'),
  ('ui.agendaEmptyStateTitle', 'en', 'All caught up!'),
  ('ui.agendaEmptyStateText', 'en', 'Your agenda for today is empty. Time to relax or plan ahead.'),

  ('navigation.agenda', 'en', 'Agenda'),
  ('navigation.projects', 'en', 'Projects'),
  ('navigation.dashboard', 'en', 'Dashboard'),
  ('navigation.users', 'en', 'Users'),
  ('navigation.configuration', 'en', 'Configuration'),
  ('navigation.jobs', 'en', 'Jobs'),
  ('navigation.userSettings', 'en', 'User Settings'),
  ('navigation.administration', 'en', 'Administration'),
  ('navigation.support', 'en', 'Support'),
  ('navigation.logout', 'en', 'Log out')
on conflict (key, language_code) do update set value = excluded.value;
