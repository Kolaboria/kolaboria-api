BEGIN;

ALTER TABLE project_applications
    ADD COLUMN reviewer_note TEXT,
    ADD COLUMN reviewed_by UUID REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN reviewed_at TIMESTAMPTZ,
    ADD COLUMN withdrawn_at TIMESTAMPTZ;

ALTER TABLE project_applications
    ADD CONSTRAINT project_applications_reviewer_note_length_check
        CHECK (reviewer_note IS NULL OR char_length(reviewer_note) <= 2000) NOT VALID,
    ADD CONSTRAINT project_applications_decision_metadata_check
        CHECK (
            (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL AND reviewer_note IS NULL AND withdrawn_at IS NULL)
            OR
            (status IN ('accepted', 'rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL AND withdrawn_at IS NULL)
            OR
            (status = 'withdrawn' AND reviewed_by IS NULL AND reviewed_at IS NULL AND reviewer_note IS NULL AND withdrawn_at IS NOT NULL)
        ) NOT VALID;

CREATE INDEX project_applications_latest_rejected_idx
    ON project_applications(project_id, applicant_user_id, applied_at DESC, id ASC)
    WHERE status = 'rejected';

ALTER TABLE projects DROP CONSTRAINT projects_creation_identity_check;
ALTER TABLE projects
    ADD CONSTRAINT projects_creation_identity_check CHECK (
        (creation_mode = 'personal' AND initiator_user_id IS NOT NULL AND initiator_organization_id IS NULL AND owner_id = initiator_user_id AND created_by_user_id = initiator_user_id)
        OR
        (creation_mode = 'organization_initiated' AND initiator_user_id IS NULL AND initiator_organization_id IS NOT NULL)
    );

COMMIT;
