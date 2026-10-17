-- Refuse a lossy downgrade rather than silently truncating historical names.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM workspace_project_context_snapshots
        WHERE char_length(initiator_organization_name) > 160
    ) THEN
        RAISE EXCEPTION 'Cannot narrow workspace organization names while snapshots exceed 160 characters';
    END IF;
END $$;

ALTER TABLE workspace_project_context_snapshots
    ALTER COLUMN initiator_organization_name TYPE VARCHAR(160);
