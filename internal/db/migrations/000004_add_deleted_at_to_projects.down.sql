DROP INDEX IF EXISTS idx_projects_user_deleted_at;

ALTER TABLE projects
    DROP COLUMN IF EXISTS deleted_at;