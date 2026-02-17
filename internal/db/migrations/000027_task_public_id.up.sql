ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS public_id CHAR(26);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_public_id_unique
    ON tasks(public_id)
    WHERE public_id IS NOT NULL;
