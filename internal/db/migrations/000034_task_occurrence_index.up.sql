-- 1) Add the column
ALTER TABLE task_occurrences ADD COLUMN IF NOT EXISTS occurrence_index BIGINT;

-- 2) Populate the column for existing rows
WITH indexed AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY due_at, id) as rn
    FROM task_occurrences
)
UPDATE task_occurrences
SET occurrence_index = indexed.rn
FROM indexed
WHERE task_occurrences.id = indexed.id;

-- 3) Make it NOT NULL and add a unique constraint
ALTER TABLE task_occurrences ALTER COLUMN occurrence_index SET NOT NULL;
ALTER TABLE task_occurrences ADD CONSTRAINT ux_task_occurrences_task_index UNIQUE (task_id, occurrence_index);
