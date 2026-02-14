insert into config_keys (key, description, data_type, is_public) values
  ('updatedSuccess', 'Success message for updated content', 'string', true),
  ('deleteAccountButton', 'Button text to delete account', 'string', true),
  ('deleteAccountTitle', 'Title for delete account dialog', 'string', true),
  ('deleteAccountPermanent', 'Warning that account deletion is permanent', 'string', true),
  ('deleteAccountConfirmation', 'Confirmation message for deleting account', 'string', true),
  ('confirmPasswordLabel', 'Label for confirm password input', 'string', true),
  ('enterPasswordPlaceholder', 'Placeholder for password input', 'string', true),
  ('deleteAccountAgreement', 'Agreement text for deleting account', 'string', true),
  ('noUsersFound', 'Message when no users are found', 'string', true),
  ('noUsersFoundDescription', 'Description when no users are found', 'string', true),
  ('cannotInactivateSelf', 'Message when user cannot inactivate self', 'string', true),
  ('inactivateWarning', 'Warning that user will be logged out and blocked', 'string', true),
  ('addLanguageTitle', 'Title for add language dialog', 'string', true),
  ('addLanguageSubtitle', 'Subtitle for add language dialog', 'string', true),
  ('languageCodeLabel', 'Label for language code input', 'string', true),
  ('displayNameLabel', 'Label for display name input', 'string', true),
  ('addingLanguage', 'Loading text when adding a language', 'string', true),
  ('addLanguageButton', 'Button text to add a language', 'string', true)
on conflict (key) do nothing;

-- en translations
insert into config_translations (key, language_code, value) values
  ('updatedSuccess', 'en', 'Updated'),
  ('deleteAccountButton', 'en', 'Delete my Account'),
  ('deleteAccountTitle', 'en', 'Delete Account'),
  ('deleteAccountPermanent', 'en', 'This action is permanent'),
  ('deleteAccountConfirmation', 'en', 'Are you absolutely sure? All your projects, tasks, and data will be permanently deleted. This cannot be undone.'),
  ('confirmPasswordLabel', 'en', 'Confirm Password'),
  ('enterPasswordPlaceholder', 'en', 'Enter your password'),
  ('deleteAccountAgreement', 'en', 'I understand that my account and all associated data will be permanently removed.'),
  ('noUsersFound', 'en', 'No users found'),
  ('noUsersFoundDescription', 'en', 'Try adjusting your filters or search terms.'),
  ('cannotInactivateSelf', 'en', 'You cannot inactivate your own account.'),
  ('inactivateWarning', 'en', 'This will immediately log the user out and block future logins.'),
  ('addLanguageTitle', 'en', 'Add Language'),
  ('addLanguageSubtitle', 'en', 'Create a new localization'),
  ('languageCodeLabel', 'en', 'Language Code (e.g. en, pt-br)'),
  ('displayNameLabel', 'en', 'Display Name'),
  ('addingLanguage', 'en', 'Adding...'),
  ('addLanguageButton', 'en', 'Add')
on conflict (key, language_code) do update set value = excluded.value;

-- fr translations
insert into config_translations (key, language_code, value) values
  ('updatedSuccess', 'fr', 'Mis à jour'),
  ('deleteAccountButton', 'fr', 'Supprimer mon compte'),
  ('deleteAccountTitle', 'fr', 'Supprimer le compte'),
  ('deleteAccountPermanent', 'fr', 'Cette action est définitive'),
  ('deleteAccountConfirmation', 'fr', 'Êtes-vous absolument sûr ? Tous vos projets, tâches et données seront définitivement supprimés. Ceci ne peut pas être annulé.'),
  ('confirmPasswordLabel', 'fr', 'Confirmer le mot de passe'),
  ('enterPasswordPlaceholder', 'fr', 'Saisissez votre mot de passe'),
  ('deleteAccountAgreement', 'fr', 'Je comprends que mon compte et toutes les données associées seront définitivement supprimés.'),
  ('noUsersFound', 'fr', 'Aucun utilisateur trouvé'),
  ('noUsersFoundDescription', 'fr', 'Essayez d''ajuster vos filtres ou vos termes de recherche.'),
  ('cannotInactivateSelf', 'fr', 'Vous ne pouvez pas désactiver votre propre compte.'),
  ('inactivateWarning', 'fr', 'Cela déconnectera immédiatement l''utilisateur et bloquera les connexions futures.'),
  ('addLanguageTitle', 'fr', 'Ajouter une langue'),
  ('addLanguageSubtitle', 'fr', 'Créer une nouvelle localisation'),
  ('languageCodeLabel', 'fr', 'Code de langue (ex. en, pt-br)'),
  ('displayNameLabel', 'fr', 'Nom affiché'),
  ('addingLanguage', 'fr', 'Ajout...'),
  ('addLanguageButton', 'fr', 'Ajouter')
on conflict (key, language_code) do update set value = excluded.value;

-- de translations
insert into config_translations (key, language_code, value) values
  ('updatedSuccess', 'de', 'Aktualisiert'),
  ('deleteAccountButton', 'de', 'Mein Konto löschen'),
  ('deleteAccountTitle', 'de', 'Konto löschen'),
  ('deleteAccountPermanent', 'de', 'Diese Aktion ist dauerhaft'),
  ('deleteAccountConfirmation', 'de', 'Sind Sie absolut sicher? Alle Ihre Projekte, Aufgaben und Daten werden dauerhaft gelöscht. Dies kann nicht rückgängig gemacht werden.'),
  ('confirmPasswordLabel', 'de', 'Passwort bestätigen'),
  ('enterPasswordPlaceholder', 'de', 'Geben Sie Ihr Passwort ein'),
  ('deleteAccountAgreement', 'de', 'Ich verstehe, dass mein Konto und alle zugehörigen Daten dauerhaft entfernt werden.'),
  ('noUsersFound', 'de', 'Keine Benutzer gefunden'),
  ('noUsersFoundDescription', 'de', 'Versuchen Sie, Ihre Filter oder Suchbegriffe anzupassen.'),
  ('cannotInactivateSelf', 'de', 'Sie können Ihr eigenes Konto nicht deaktivieren.'),
  ('inactivateWarning', 'de', 'Dadurch wird der Benutzer sofort abgemeldet und zukünftige Anmeldungen werden blockiert.'),
  ('addLanguageTitle', 'de', 'Sprache hinzufügen'),
  ('addLanguageSubtitle', 'de', 'Eine neue Lokalisierung erstellen'),
  ('languageCodeLabel', 'de', 'Sprachcode (z. B. en, pt-br)'),
  ('displayNameLabel', 'de', 'Anzeigename'),
  ('addingLanguage', 'de', 'Hinzufügen...'),
  ('addLanguageButton', 'de', 'Hinzufügen')
on conflict (key, language_code) do update set value = excluded.value;
