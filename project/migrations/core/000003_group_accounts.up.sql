-- Corporate-account groups: a group is a shared workspace whose members can see
-- each other's plans and documents ("all members share everything"). The base
-- `groups` and `user_group_relations` tables already exist; here we give a group
-- a human name and an owner (the creator), and add the membership lookup indexes
-- the access checks rely on.
ALTER TABLE groups ADD COLUMN IF NOT EXISTS name VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE groups ADD COLUMN IF NOT EXISTS created_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL;

-- `role` and `description` are NOT NULL with no default in the original schema;
-- give them defaults so a corporate account can be created with just a name.
ALTER TABLE groups ALTER COLUMN role SET DEFAULT 'corporate';
ALTER TABLE groups ALTER COLUMN description SET DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_groups_created_by ON groups(created_by);
