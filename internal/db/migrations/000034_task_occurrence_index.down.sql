ALTER TABLE task_occurrences DROP CONSTRAINT IF EXISTS ux_task_occurrences_task_index;
ALTER TABLE task_occurrences DROP COLUMN IF EXISTS occurrence_index;
