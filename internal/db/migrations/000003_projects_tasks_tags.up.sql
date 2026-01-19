-- Projects
CREATE TABLE IF NOT EXISTS projects (
                                        id         BIGSERIAL PRIMARY KEY,
                                        user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                        name       TEXT NOT NULL,
                                        description TEXT,
                                        created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                                        updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                                        UNIQUE (id, user_id),
                                        UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_projects_user_id ON projects(user_id);

-- Tasks
CREATE TABLE IF NOT EXISTS tasks (
                                     id           BIGSERIAL PRIMARY KEY,
                                     user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                     project_id   BIGINT NOT NULL,
                                     title        TEXT NOT NULL,
                                     description  TEXT,
                                     due_at       TIMESTAMPTZ,
                                     completed_at TIMESTAMPTZ,
                                     deleted_at   TIMESTAMPTZ,

    -- Recurrence (MVP)
                                     repeat_every INTEGER,
                                     repeat_unit  TEXT,

                                     created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
                                     updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Ensure task's user_id matches the owning project user_id
                                     CONSTRAINT fk_tasks_project_owner
                                         FOREIGN KEY (project_id, user_id)
                                             REFERENCES projects(id, user_id)
                                             ON DELETE CASCADE,

    -- recurrence validity
                                     CONSTRAINT chk_tasks_repeat
                                         CHECK (
                                             (repeat_every IS NULL AND repeat_unit IS NULL)
                                                 OR
                                             (repeat_every IS NOT NULL AND repeat_every > 0 AND repeat_unit IN ('day','week','month'))
                                             ),

                                     UNIQUE (id, user_id)
);

-- Common query indexes
CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_user_open ON tasks(user_id) WHERE deleted_at IS NULL AND completed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_due_at ON tasks(due_at) WHERE deleted_at IS NULL AND completed_at IS NULL;

-- Tags
CREATE TABLE IF NOT EXISTS tags (
                                    id         BIGSERIAL PRIMARY KEY,
                                    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                    name       TEXT NOT NULL,
                                    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                                    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                                    UNIQUE (id, user_id),
                                    UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_tags_user_id ON tags(user_id);

-- Task <-> Tags (M:N)
CREATE TABLE IF NOT EXISTS task_tags (
                                         task_id    BIGINT NOT NULL,
                                         tag_id     BIGINT NOT NULL,
                                         user_id    BIGINT NOT NULL,
                                         created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

                                         PRIMARY KEY (task_id, tag_id),

    -- Enforce that both the task and tag belong to the same user
                                         CONSTRAINT fk_task_tags_task
                                             FOREIGN KEY (task_id, user_id)
                                                 REFERENCES tasks(id, user_id)
                                                 ON DELETE CASCADE,

                                         CONSTRAINT fk_task_tags_tag
                                             FOREIGN KEY (tag_id, user_id)
                                                 REFERENCES tags(id, user_id)
                                                 ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_task_tags_user_id ON task_tags(user_id);
CREATE INDEX IF NOT EXISTS idx_task_tags_tag_id ON task_tags(tag_id);
