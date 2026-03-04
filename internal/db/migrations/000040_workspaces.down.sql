-- Reverse Workspaces and Organizations support

-- 1. Restore user_id columns
ALTER TABLE projects ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE tasks ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE tags ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE task_tags ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE task_occurrences ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;

-- 2. Backfill user_id from workspace_id (only works for user workspaces)
UPDATE projects p SET user_id = w.user_id FROM workspaces w WHERE p.workspace_id = w.id;
UPDATE tasks t SET user_id = w.user_id FROM workspaces w WHERE t.workspace_id = w.id;
UPDATE tags t SET user_id = w.user_id FROM workspaces w WHERE t.workspace_id = w.id;
UPDATE task_tags tt SET user_id = w.user_id FROM workspaces w WHERE tt.workspace_id = w.id;
UPDATE task_occurrences toc SET user_id = w.user_id FROM workspaces w WHERE toc.workspace_id = w.id;

-- 3. Set NOT NULL
ALTER TABLE projects ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE tasks ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE tags ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE task_tags ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE task_occurrences ALTER COLUMN user_id SET NOT NULL;

-- 4. Restore original unique constraints and indexes

-- 4.1 Drop workspace-based dependent foreign keys FIRST
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS fk_tasks_project_owner_workspace;
ALTER TABLE task_tags DROP CONSTRAINT IF EXISTS fk_task_tags_task_workspace;
ALTER TABLE task_tags DROP CONSTRAINT IF EXISTS fk_task_tags_tag_workspace;

-- 4.2 Drop workspace-based unique constraints
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_workspace_id_name_key;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_id_workspace_id_key;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_id_workspace_id_key;
ALTER TABLE tags DROP CONSTRAINT IF EXISTS tags_workspace_id_name_key;
ALTER TABLE tags DROP CONSTRAINT IF EXISTS tags_id_workspace_id_key;

-- 4.3 Drop simple workspace-based FKs
ALTER TABLE projects DROP CONSTRAINT IF EXISTS fk_projects_workspace;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS fk_tasks_workspace;
ALTER TABLE tags DROP CONSTRAINT IF EXISTS fk_tags_workspace;
ALTER TABLE task_tags DROP CONSTRAINT IF EXISTS fk_task_tags_workspace;
ALTER TABLE task_occurrences DROP CONSTRAINT IF EXISTS fk_task_occurrences_workspace;

-- 4.4 Add back user-based unique constraints
ALTER TABLE projects ADD CONSTRAINT projects_user_id_name_key UNIQUE (user_id, name);
ALTER TABLE projects ADD CONSTRAINT projects_id_user_id_key UNIQUE (id, user_id);
ALTER TABLE tasks ADD CONSTRAINT tasks_id_user_id_key UNIQUE (id, user_id);
ALTER TABLE tags ADD CONSTRAINT tags_user_id_name_key UNIQUE (user_id, name);
ALTER TABLE tags ADD CONSTRAINT tags_id_user_id_key UNIQUE (id, user_id);

-- 4.5 Add back user-based foreign keys
ALTER TABLE tasks ADD CONSTRAINT fk_tasks_project_owner
    FOREIGN KEY (project_id, user_id) REFERENCES projects(id, user_id) ON DELETE CASCADE;
ALTER TABLE task_tags ADD CONSTRAINT fk_task_tags_task
    FOREIGN KEY (task_id, user_id) REFERENCES tasks(id, user_id) ON DELETE CASCADE;
ALTER TABLE task_tags ADD CONSTRAINT fk_task_tags_tag
    FOREIGN KEY (tag_id, user_id) REFERENCES tags(id, user_id) ON DELETE CASCADE;

-- 5. Restore user_id based indexes
DROP INDEX IF EXISTS idx_projects_workspace_id;
CREATE INDEX idx_projects_user_id ON projects(user_id);

DROP INDEX IF EXISTS idx_tasks_workspace_id;
CREATE INDEX idx_tasks_user_id ON tasks(user_id);

DROP INDEX IF EXISTS idx_tags_workspace_id;
CREATE INDEX idx_tags_user_id ON tags(user_id);

DROP INDEX IF EXISTS idx_task_tags_workspace_id;
CREATE INDEX idx_task_tags_user_id ON task_tags(user_id);

DROP INDEX IF EXISTS idx_task_occurrences_workspace_due;
CREATE INDEX idx_task_occurrences_user_due ON task_occurrences (user_id, due_at);

DROP INDEX IF EXISTS idx_tasks_workspace_next_due;
CREATE INDEX idx_tasks_user_next_due ON tasks (user_id, next_due_at);

DROP INDEX IF EXISTS idx_tasks_workspace_open;
CREATE INDEX idx_tasks_user_open ON tasks(user_id) WHERE deleted_at IS NULL AND completed_at IS NULL;

-- 6. Drop workspace_id columns and workspace/org tables
ALTER TABLE projects DROP COLUMN workspace_id;
ALTER TABLE tasks DROP COLUMN workspace_id;
ALTER TABLE tags DROP COLUMN workspace_id;
ALTER TABLE task_tags DROP COLUMN workspace_id;
ALTER TABLE task_occurrences DROP COLUMN workspace_id;

DROP TABLE workspaces;
DROP TABLE orgs;
DROP TYPE workspace_type;
