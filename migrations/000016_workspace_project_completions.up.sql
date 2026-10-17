CREATE TABLE workspace_project_completions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE RESTRICT,
    completed_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    completed_at TIMESTAMPTZ NOT NULL,
    source_project_version BIGINT NOT NULL CHECK (source_project_version > 0),
    project_version BIGINT NOT NULL,
    task_version BIGINT NOT NULL CHECK (task_version >= 0),
    total_tasks BIGINT NOT NULL CHECK (total_tasks >= 0),
    done_tasks BIGINT NOT NULL CHECK (done_tasks >= 0),
    closed_tasks BIGINT NOT NULL CHECK (closed_tasks >= 0),
    deleted_tasks BIGINT NOT NULL CHECK (deleted_tasks >= 0),
    result_summary TEXT NOT NULL CHECK (char_length(btrim(result_summary)) BETWEEN 1 AND 5000),
    review_basis TEXT NOT NULL DEFAULT '' CHECK (char_length(review_basis) <= 5000),
    CHECK (project_version = source_project_version + 1),
    CHECK (total_tasks = done_tasks + closed_tasks),
    CHECK (total_tasks > 0 OR char_length(btrim(review_basis)) > 0)
);

-- Closure summaries include tombstones, which the partial board index excludes.
CREATE INDEX workspace_tasks_project_summary_idx ON workspace_tasks(project_id);
