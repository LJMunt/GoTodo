ALTER TABLE users
ADD COLUMN ui_theme TEXT not null default 'system',
ADD COLUMN show_completed_default BOOLEAN not null default false,
ADD COLUMN last_login TIMESTAMPTZ;

ALTER TABLE users
ADD CONSTRAINT ui_theme_check
CHECK (ui_theme IN ('system', 'light', 'dark'));