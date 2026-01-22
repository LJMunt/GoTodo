-- Remove uniqueness on task/tag relation
DROP INDEX IF EXISTS ux_task_tags_unique;

-- Remove user+tag uniqueness
DROP INDEX IF EXISTS ux_tags_user_name;

-- Restore original foreign key without cascade
ALTER TABLE task_tags
    DROP CONSTRAINT IF EXISTS task_tags_tag_id_fkey;

ALTER TABLE task_tags
    ADD CONSTRAINT task_tags_tag_id_fkey
        FOREIGN KEY (tag_id)
            REFERENCES tags(id);

-- Convert name back to text
ALTER TABLE tags
    ALTER COLUMN name TYPE text;

-- Optional: remove extension
-- (only do this if nothing else uses citext)
DROP EXTENSION IF EXISTS citext;
