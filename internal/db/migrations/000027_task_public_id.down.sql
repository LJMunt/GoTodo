DROP INDEX IF EXISTS idx_tasks_public_id_unique;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS public_id;
