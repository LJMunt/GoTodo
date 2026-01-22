ALTER TABLE tasks
    DROP CONSTRAINT IF EXISTS tasks_repeat_fields_consistency;

DROP INDEX IF EXISTS idx_tasks_user_next_due;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS next_due_at,
    DROP COLUMN IF EXISTS recurrence_start_at;

DROP INDEX IF EXISTS idx_task_occurrences_user_due;
DROP INDEX IF EXISTS idx_task_occurrences_task_due;
DROP INDEX IF EXISTS ux_task_occurrences_task_due;

DROP TABLE IF EXISTS task_occurrences;
