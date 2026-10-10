CREATE TABLE project_applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    project_role_id UUID NOT NULL,
    applicant_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    motivation TEXT NOT NULL CHECK (char_length(btrim(motivation)) >= 10),
    expected_contribution TEXT,
    portfolio_links JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(portfolio_links) = 'array'),
    availability VARCHAR(24) NOT NULL CHECK (availability IN ('flexible', 'full_time', 'part_time', 'weekends_only')),
    estimated_hours_per_week INTEGER CHECK (estimated_hours_per_week IS NULL OR estimated_hours_per_week BETWEEN 1 AND 168),
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'withdrawn')),
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_applications_role_project_fk
        FOREIGN KEY (project_role_id, project_id)
        REFERENCES project_roles(id, project_id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX project_applications_one_active_per_user_idx
    ON project_applications(project_id, applicant_user_id)
    WHERE status IN ('pending', 'accepted');
CREATE INDEX project_applications_project_status_idx
    ON project_applications(project_id, status, applied_at DESC);
CREATE INDEX project_applications_applicant_idx
    ON project_applications(applicant_user_id, applied_at DESC);
