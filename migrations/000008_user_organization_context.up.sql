ALTER TABLE users
    ADD COLUMN system_role VARCHAR(20) NOT NULL DEFAULT 'user',
    ADD CONSTRAINT users_system_role_check CHECK (system_role IN ('user', 'admin'));

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL CHECK (length(btrim(name)) > 0),
    slug VARCHAR(100) NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE organization_members (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    can_initiate_projects BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, user_id)
);

CREATE INDEX organization_members_initiable_user_idx
    ON organization_members(user_id, organization_id)
    WHERE is_active AND can_initiate_projects;

-- Organization existence grants no user privileges or memberships.
INSERT INTO organizations (id, name, slug)
VALUES ('bc9e7805-6000-4e0e-9d00-95fc8b85fe6a', 'Kolaboria', 'kolaboria');
