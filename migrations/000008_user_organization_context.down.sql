DROP TABLE organization_members;
DROP TABLE organizations;
ALTER TABLE users DROP CONSTRAINT users_system_role_check, DROP COLUMN system_role;
