-- Workspaces and Organizations support

CREATE TYPE workspace_type AS ENUM ('user', 'org');

-- Organizations table
CREATE TABLE orgs (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Workspaces table
CREATE TABLE workspaces (
    id         BIGSERIAL PRIMARY KEY,
    public_id  CHAR(26) UNIQUE NOT NULL,
    type       workspace_type NOT NULL,
    user_id    BIGINT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id     BIGINT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Exactly one owner reference must be set and must match the type
    CONSTRAINT workspaces_owner_check CHECK (
        (type = 'user' AND user_id IS NOT NULL AND org_id IS NULL)
        OR (type = 'org' AND org_id IS NOT NULL AND user_id IS NULL)
    )
);

-- Enforce "one personal workspace per user"
CREATE UNIQUE INDEX workspaces_unique_user ON workspaces(user_id) WHERE type = 'user';
-- Enforce "one workspace per org"
CREATE UNIQUE INDEX workspaces_unique_org ON workspaces(org_id) WHERE type = 'org';

-- 1. Backfill: Create a personal workspace for every existing user
-- For backfill, we generate a public_id based on the user's public_id but prefixed differently
-- We assume users already have a 26-char public_id.
INSERT INTO workspaces (type, user_id, public_id, created_at, updated_at)
SELECT 'user', id, '01' || substr(public_id, 3), created_at, updated_at FROM users;

-- 2. Add workspace_id columns to owned objects
ALTER TABLE projects ADD COLUMN workspace_id BIGINT;
ALTER TABLE tasks ADD COLUMN workspace_id BIGINT;
ALTER TABLE tags ADD COLUMN workspace_id BIGINT;
ALTER TABLE task_tags ADD COLUMN workspace_id BIGINT;
ALTER TABLE task_occurrences ADD COLUMN workspace_id BIGINT;

-- 3. Backfill ownership transfer
UPDATE projects p SET workspace_id = w.id FROM workspaces w WHERE p.user_id = w.user_id AND w.type = 'user';
UPDATE tasks t SET workspace_id = w.id FROM workspaces w WHERE t.user_id = w.user_id AND w.type = 'user';
UPDATE tags t SET workspace_id = w.id FROM workspaces w WHERE t.user_id = w.user_id AND w.type = 'user';
UPDATE task_tags tt SET workspace_id = w.id FROM workspaces w WHERE tt.user_id = w.user_id AND w.type = 'user';
UPDATE task_occurrences toc SET workspace_id = w.id FROM workspaces w WHERE toc.user_id = w.user_id AND w.type = 'user';

-- 4. Set NOT NULL on workspace_id columns
ALTER TABLE projects ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE tasks ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE tags ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE task_tags ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE task_occurrences ALTER COLUMN workspace_id SET NOT NULL;

-- 5. Add FK constraints for workspace_id
ALTER TABLE projects ADD CONSTRAINT fk_projects_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE tasks ADD CONSTRAINT fk_tasks_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE tags ADD CONSTRAINT fk_tags_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE task_tags ADD CONSTRAINT fk_task_tags_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE task_occurrences ADD CONSTRAINT fk_task_occurrences_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;

-- 6. Update unique constraints and indexes to use workspace_id instead of user_id

-- 6.1 Drop all dependent foreign keys FIRST
ALTER TABLE task_tags DROP CONSTRAINT IF EXISTS fk_task_tags_task;
ALTER TABLE task_tags DROP CONSTRAINT IF EXISTS fk_task_tags_tag;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS fk_tasks_project_owner;

-- 6.2 Drop unique constraints that are now safe to drop
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_user_id_name_key;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_id_user_id_key;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_id_user_id_key;
ALTER TABLE tags DROP CONSTRAINT IF EXISTS tags_user_id_name_key;
ALTER TABLE tags DROP CONSTRAINT IF EXISTS tags_id_user_id_key;

-- 6.3 Add new workspace-based unique constraints
ALTER TABLE projects ADD CONSTRAINT projects_workspace_id_name_key UNIQUE (workspace_id, name);
ALTER TABLE projects ADD CONSTRAINT projects_id_workspace_id_key UNIQUE (id, workspace_id);
ALTER TABLE tasks ADD CONSTRAINT tasks_id_workspace_id_key UNIQUE (id, workspace_id);
ALTER TABLE tags ADD CONSTRAINT tags_workspace_id_name_key UNIQUE (workspace_id, name);
ALTER TABLE tags ADD CONSTRAINT tags_id_workspace_id_key UNIQUE (id, workspace_id);

-- 6.4 Add new workspace-based foreign keys
ALTER TABLE tasks ADD CONSTRAINT fk_tasks_project_owner_workspace
    FOREIGN KEY (project_id, workspace_id) REFERENCES projects(id, workspace_id) ON DELETE CASCADE;
ALTER TABLE task_tags ADD CONSTRAINT fk_task_tags_task_workspace
    FOREIGN KEY (task_id, workspace_id) REFERENCES tasks(id, workspace_id) ON DELETE CASCADE;
ALTER TABLE task_tags ADD CONSTRAINT fk_task_tags_tag_workspace
    FOREIGN KEY (tag_id, workspace_id) REFERENCES tags(id, workspace_id) ON DELETE CASCADE;

-- 7. Replace user_id based indexes with workspace_id based ones
DROP INDEX IF EXISTS idx_projects_user_id;
CREATE INDEX idx_projects_workspace_id ON projects(workspace_id);

DROP INDEX IF EXISTS idx_tasks_user_id;
CREATE INDEX idx_tasks_workspace_id ON tasks(workspace_id);

DROP INDEX IF EXISTS idx_tags_user_id;
CREATE INDEX idx_tags_workspace_id ON tags(workspace_id);

DROP INDEX IF EXISTS idx_task_tags_user_id;
CREATE INDEX idx_task_tags_workspace_id ON task_tags(workspace_id);

DROP INDEX IF EXISTS idx_task_occurrences_user_due;
CREATE INDEX idx_task_occurrences_workspace_due ON task_occurrences (workspace_id, due_at);

DROP INDEX IF EXISTS idx_tasks_user_next_due;
CREATE INDEX idx_tasks_workspace_next_due ON tasks (workspace_id, next_due_at);

-- Update partial indexes
DROP INDEX IF EXISTS idx_tasks_user_open;
CREATE INDEX idx_tasks_workspace_open ON tasks(workspace_id) WHERE deleted_at IS NULL AND completed_at IS NULL;

-- 8. Finally drop the user_id columns from projects, tasks, tags, task_tags and task_occurrences
ALTER TABLE projects DROP COLUMN user_id;
ALTER TABLE tasks DROP COLUMN user_id;
ALTER TABLE tags DROP COLUMN user_id;
ALTER TABLE task_tags DROP COLUMN user_id;
ALTER TABLE task_occurrences DROP COLUMN user_id;
