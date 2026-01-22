ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_projects_user_deleted_at
    ON projects (user_id, deleted_at);
