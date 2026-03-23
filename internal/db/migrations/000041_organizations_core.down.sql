-- Package 1 down migration

-- 1. Remove task attribution fields
ALTER TABLE tasks DROP COLUMN created_by;
ALTER TABLE tasks DROP COLUMN closed_by;
ALTER TABLE tasks DROP COLUMN assigned_to;

-- 2. Remove org_members table
DROP TABLE IF EXISTS org_members;

-- 3. Remove deleted_at from orgs
ALTER TABLE orgs DROP COLUMN deleted_at;
