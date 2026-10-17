-- Organization names permit 255 characters. Preserve the full valid source
-- name so recording history never rejects a legitimate domain operation.
ALTER TABLE workspace_project_context_snapshots
    ALTER COLUMN initiator_organization_name TYPE VARCHAR(255);
