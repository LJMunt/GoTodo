-- seed fr translations
insert into config_translations (key, language_code, value)
values
  ('branding.appName', 'fr', 'Gotodo'),
  ('branding.appLogoInitial', 'fr', 'G'),
  ('branding.supportEmail', 'fr', 'support@todexia.app'),

  ('auth.loginTitle', 'fr', 'Bon retour'),
  ('auth.loginSubtitle', 'fr', 'Reprenez votre agenda.'),
  ('auth.signupTitle', 'fr', 'Créer un compte'),
  ('auth.signupSubtitle', 'fr', 'Créez un espace de travail qui vous ressemble.'),

  ('ui.agendaTitle', 'fr', 'Votre Agenda'),
  ('ui.agendaEmptyStateTitle', 'fr', 'Tout est à jour !'),
  ('ui.agendaEmptyStateText', 'fr', 'Votre agenda pour aujourd''hui est vide. C''est le moment de se détendre ou d''anticiper.'),

  ('navigation.agenda', 'fr', 'Agenda'),
  ('navigation.projects', 'fr', 'Projets'),
  ('navigation.dashboard', 'fr', 'Tableau de bord'),
  ('navigation.users', 'fr', 'Utilisateurs'),
  ('navigation.configuration', 'fr', 'Configuration'),
  ('navigation.jobs', 'fr', 'Tâches planifiées'),
  ('navigation.userSettings', 'fr', 'Paramètres utilisateur'),
  ('navigation.administration', 'fr', 'Administration'),
  ('navigation.support', 'fr', 'Support'),
  ('navigation.logout', 'fr', 'Déconnexion')
on conflict (key, language_code) do update set value = excluded.value;

-- seed de translations
insert into config_translations (key, language_code, value)
values
  ('branding.appName', 'de', 'Gotodo'),
  ('branding.appLogoInitial', 'de', 'G'),
  ('branding.supportEmail', 'de', 'support@todexia.app'),

  ('auth.loginTitle', 'de', 'Willkommen zurück'),
  ('auth.loginSubtitle', 'de', 'Kehren Sie zu Ihrer Agenda zurück.'),
  ('auth.signupTitle', 'de', 'Konto erstellen'),
  ('auth.signupSubtitle', 'de', 'Erstellen Sie einen Arbeitsbereich, der Ihnen gehört.'),

  ('ui.agendaTitle', 'de', 'Ihre Agenda'),
  ('ui.agendaEmptyStateTitle', 'de', 'Alles erledigt!'),
  ('ui.agendaEmptyStateText', 'de', 'Ihre Agenda für heute ist leer. Zeit zum Entspannen oder Vorausplanen.'),

  ('navigation.agenda', 'de', 'Agenda'),
  ('navigation.projects', 'de', 'Projekte'),
  ('navigation.dashboard', 'de', 'Dashboard'),
  ('navigation.users', 'de', 'Benutzer'),
  ('navigation.configuration', 'de', 'Konfiguration'),
  ('navigation.jobs', 'de', 'Jobs'),
  ('navigation.userSettings', 'de', 'Benutzereinstellungen'),
  ('navigation.administration', 'de', 'Administration'),
  ('navigation.support', 'de', 'Support'),
  ('navigation.logout', 'de', 'Abmelden')
on conflict (key, language_code) do update set value = excluded.value;
