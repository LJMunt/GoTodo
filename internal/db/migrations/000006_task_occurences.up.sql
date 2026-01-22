-- 1) Create task_occurrences table (history of scheduled instances)
CREATE TABLE IF NOT EXISTS task_occurrences (
                                                id           BIGSERIAL PRIMARY KEY,
                                                user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                                task_id      BIGINT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

                                                due_at       TIMESTAMPTZ NOT NULL,
                                                completed_at TIMESTAMPTZ NULL,

                                                created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
                                                updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_task_occurrences_task_due
    ON task_occurrences (task_id, due_at);

CREATE INDEX IF NOT EXISTS idx_task_occurrences_task_due
    ON task_occurrences (task_id, due_at);

CREATE INDEX IF NOT EXISTS idx_task_occurrences_user_due
    ON task_occurrences (user_id, due_at);


-- 2) Adjust tasks table to support recurrence templates
ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS recurrence_start_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS next_due_at          TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_user_next_due
    ON tasks (user_id, next_due_at);

ALTER TABLE tasks
    ADD CONSTRAINT tasks_repeat_fields_consistency
        CHECK (
            (repeat_every IS NULL AND repeat_unit IS NULL)
                OR
            (repeat_every IS NOT NULL AND repeat_unit IS NOT NULL AND repeat_every > 0)
            );
