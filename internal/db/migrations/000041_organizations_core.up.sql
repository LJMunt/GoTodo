-- 1. Extend orgs table with deleted_at for soft deletion
ALTER TABLE orgs ADD COLUMN deleted_at TIMESTAMPTZ;

-- 2. Create org_members table
CREATE TABLE org_members (
    org_id     BIGINT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT org_members_pkey PRIMARY KEY (org_id, user_id),
    CONSTRAINT org_members_role_check CHECK (role IN ('admin', 'member'))
);

CREATE INDEX idx_org_members_user_id ON org_members(user_id);

-- 3. Add task attribution fields
ALTER TABLE tasks ADD COLUMN created_by BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN closed_by BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN assigned_to BIGINT REFERENCES users(id) ON DELETE SET NULL;

-- 4. Backfill attribution for existing tasks
-- We assume all existing tasks were created and assigned to the owner of the personal workspace they belong to.
-- Also for closed tasks, the closer is the owner.
UPDATE tasks t
SET created_by = w.user_id,
    assigned_to = w.user_id,
    closed_by = CASE WHEN t.completed_at IS NOT NULL THEN w.user_id ELSE NULL END
FROM workspaces w
WHERE t.workspace_id = w.id AND w.type = 'user';
