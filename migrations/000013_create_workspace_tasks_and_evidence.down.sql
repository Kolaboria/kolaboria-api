DROP TABLE IF EXISTS workspace_activity_events;
DROP TABLE IF EXISTS workspace_task_comments;
DROP TABLE IF EXISTS workspace_task_review_cycles;
DROP TABLE IF EXISTS workspace_task_revisions;
DROP TABLE IF EXISTS workspace_tasks;
DROP TABLE IF EXISTS workspace_project_context_snapshots;

ALTER TABLE project_members
    DROP CONSTRAINT IF EXISTS project_members_id_project_unique,
    DROP CONSTRAINT IF EXISTS project_members_task_access_version_check,
    DROP COLUMN IF EXISTS task_access_version,
    DROP COLUMN IF EXISTS task_management_granted;
