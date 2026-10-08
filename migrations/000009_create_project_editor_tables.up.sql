CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(100) NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    creation_mode VARCHAR(32) NOT NULL CHECK (creation_mode IN ('personal', 'organization_initiated')),
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    initiator_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    initiator_organization_id UUID REFERENCES organizations(id) ON DELETE RESTRICT,
    owner_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    title VARCHAR(80) NOT NULL DEFAULT '',
    summary VARCHAR(180) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 20000),
    project_category VARCHAR(32) NOT NULL DEFAULT 'product'
        CHECK (project_category IN ('product', 'community', 'open_source', 'research', 'education', 'business', 'other')),
    visibility VARCHAR(16) NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private', 'invite_only')),
    origin VARCHAR(16) NOT NULL DEFAULT 'personal' CHECK (origin IN ('personal', 'community', 'experiment', 'client')),
    why_collaborative TEXT NOT NULL DEFAULT '' CHECK (char_length(why_collaborative) <= 5000),
    contributor_outcome TEXT NOT NULL DEFAULT '' CHECK (char_length(contributor_outcome) <= 5000),
    owner_commitment TEXT NOT NULL DEFAULT '' CHECK (char_length(owner_commitment) <= 5000),
    lead_expectations TEXT NOT NULL DEFAULT '' CHECK (char_length(lead_expectations) <= 5000),
    client_acknowledgement BOOLEAN NOT NULL DEFAULT false,
    start_date DATE,
    deadline DATE,
    availability VARCHAR(24) NOT NULL DEFAULT 'flexible'
        CHECK (availability IN ('flexible', 'part_time', 'weekends_only', 'full_time')),
    hours_per_week SMALLINT NOT NULL DEFAULT 5 CHECK (hours_per_week IN (5, 10, 15, 20)),
    collaboration_agreement BOOLEAN NOT NULL DEFAULT false,
    agreement_at TIMESTAMPTZ,
    status VARCHAR(24) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'open', 'awaiting_owner', 'in_progress', 'completed', 'archived')),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version >= 1),
    client_request_id UUID NOT NULL,
    create_request_hash CHAR(64) NOT NULL CHECK (create_request_hash ~ '^[0-9a-f]{64}$'),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT projects_creation_identity_check CHECK (
        (creation_mode = 'personal' AND initiator_user_id IS NOT NULL AND initiator_organization_id IS NULL AND owner_id = initiator_user_id AND created_by_user_id = initiator_user_id)
        OR
        (creation_mode = 'organization_initiated' AND initiator_user_id IS NULL AND initiator_organization_id IS NOT NULL AND owner_id IS NULL)
    ),
    CONSTRAINT projects_dates_check CHECK (start_date IS NULL OR deadline IS NULL OR deadline >= start_date)
);

CREATE UNIQUE INDEX projects_creator_request_unique_idx ON projects(created_by_user_id, client_request_id);
CREATE INDEX projects_public_discovery_idx ON projects(status, visibility, updated_at DESC, id);
CREATE INDEX projects_initiator_user_idx ON projects(initiator_user_id, updated_at DESC, id);
CREATE INDEX projects_initiator_organization_idx ON projects(initiator_organization_id, updated_at DESC, id);
CREATE INDEX projects_owner_idx ON projects(owner_id, updated_at DESC, id);

CREATE TABLE project_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    contribution_role_id UUID REFERENCES contribution_roles(id) ON DELETE RESTRICT,
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 5000),
    capacity SMALLINT NOT NULL DEFAULT 1 CHECK (capacity BETWEEN 1 AND 20),
    position SMALLINT NOT NULL CHECK (position BETWEEN 0 AND 19),
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_roles_id_project_unique UNIQUE (id, project_id)
);
CREATE UNIQUE INDEX project_roles_active_position_unique_idx ON project_roles(project_id, position)
    WHERE archived_at IS NULL;
CREATE INDEX project_roles_project_idx ON project_roles(project_id, archived_at, position);
CREATE INDEX project_roles_contribution_role_idx ON project_roles(contribution_role_id);

CREATE TABLE project_role_skills (
    project_role_id UUID NOT NULL REFERENCES project_roles(id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_role_id, skill_id)
);
CREATE INDEX project_role_skills_skill_idx ON project_role_skills(skill_id);

CREATE TABLE project_role_tools (
    project_role_id UUID NOT NULL REFERENCES project_roles(id) ON DELETE CASCADE,
    tool_id UUID NOT NULL REFERENCES tools(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_role_id, tool_id)
);
CREATE INDEX project_role_tools_tool_idx ON project_role_tools(tool_id);

CREATE TABLE project_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    membership_role VARCHAR(16) NOT NULL CHECK (membership_role IN ('owner', 'contributor')),
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'left', 'removed')),
    project_role_id UUID,
    contribution_role_id UUID REFERENCES contribution_roles(id) ON DELETE RESTRICT,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_members_role_project_fk FOREIGN KEY (project_role_id, project_id)
        REFERENCES project_roles(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT project_members_owner_shape_check CHECK (
        (membership_role = 'owner' AND project_role_id IS NULL)
        OR (membership_role = 'contributor' AND project_role_id IS NOT NULL)
    )
);
CREATE UNIQUE INDEX project_members_one_active_owner_idx ON project_members(project_id)
    WHERE membership_role = 'owner' AND status = 'active';
CREATE UNIQUE INDEX project_members_one_active_user_idx ON project_members(project_id, user_id)
    WHERE status = 'active';
CREATE INDEX project_members_role_status_idx ON project_members(project_role_id, status);
