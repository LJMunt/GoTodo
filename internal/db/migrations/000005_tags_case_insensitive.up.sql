CREATE EXTENSION IF NOT EXISTS citext;

-- noinspection SqlResolve
ALTER TABLE tags
    ALTER COLUMN name TYPE citext;

-- Enforce unique tag names per user (case-insensitive via citext)
CREATE UNIQUE INDEX IF NOT EXISTS ux_tags_user_name
    ON tags (user_id, name);

-- Ensure deleting a tag removes it from tasks
ALTER TABLE task_tags
    DROP CONSTRAINT IF EXISTS task_tags_tag_id_fkey;

ALTER TABLE task_tags
    ADD CONSTRAINT task_tags_tag_id_fkey
        FOREIGN KEY (tag_id)
            REFERENCES tags(id)
            ON DELETE CASCADE;

-- Prevent duplicate tag assignment to the same task
CREATE UNIQUE INDEX IF NOT EXISTS ux_task_tags_unique
    ON task_tags (user_id, task_id, tag_id);
