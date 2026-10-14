BEGIN;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM projects
        WHERE creation_mode = 'organization_initiated'
          AND owner_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'cannot roll back applicants migration while an organization project has a claimed owner';
    END IF;
END
$$;

ALTER TABLE projects DROP CONSTRAINT projects_creation_identity_check;
ALTER TABLE projects
    ADD CONSTRAINT projects_creation_identity_check CHECK (
        (creation_mode = 'personal' AND initiator_user_id IS NOT NULL AND initiator_organization_id IS NULL AND owner_id = initiator_user_id AND created_by_user_id = initiator_user_id)
        OR
        (creation_mode = 'organization_initiated' AND initiator_user_id IS NULL AND initiator_organization_id IS NOT NULL AND owner_id IS NULL)
    );

DROP INDEX project_applications_latest_rejected_idx;
ALTER TABLE project_applications
    DROP CONSTRAINT project_applications_decision_metadata_check,
    DROP CONSTRAINT project_applications_reviewer_note_length_check,
    DROP COLUMN withdrawn_at,
    DROP COLUMN reviewed_at,
    DROP COLUMN reviewed_by,
    DROP COLUMN reviewer_note;

COMMIT;
