ALTER TABLE project_members
    ADD COLUMN task_management_granted BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN task_access_version BIGINT NOT NULL DEFAULT 1,
    ADD CONSTRAINT project_members_task_access_version_check CHECK (task_access_version > 0),
    ADD CONSTRAINT project_members_id_project_unique UNIQUE (id, project_id);

CREATE TABLE workspace_project_context_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    project_version BIGINT NOT NULL,
    context_hash CHAR(64) NOT NULL,
    title VARCHAR(160) NOT NULL,
    summary TEXT NOT NULL,
    project_category VARCHAR(40) NOT NULL,
    initiator_organization_id UUID,
    initiator_organization_name VARCHAR(160),
    captured_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    schema_version SMALLINT NOT NULL DEFAULT 1,
    CONSTRAINT workspace_context_project_fk FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_context_id_project_unique UNIQUE (id, project_id),
    CONSTRAINT workspace_context_version_hash_unique UNIQUE (project_id, project_version, context_hash),
    CONSTRAINT workspace_context_version_check CHECK (project_version > 0),
    CONSTRAINT workspace_context_org_shape_check CHECK (
        (initiator_organization_id IS NULL AND initiator_organization_name IS NULL) OR
        (initiator_organization_id IS NOT NULL AND initiator_organization_name IS NOT NULL)
    )
);

CREATE TABLE workspace_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    title VARCHAR(160) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL DEFAULT 'todo',
    priority VARCHAR(12) NOT NULL DEFAULT 'medium',
    due_date DATE,
    assignee_member_id UUID,
    version BIGINT NOT NULL DEFAULT 1,
    content_revision BIGINT NOT NULL DEFAULT 1,
    created_by_user_id UUID NOT NULL,
    updated_by_user_id UUID NOT NULL,
    completed_at TIMESTAMPTZ,
    completed_by_user_id UUID,
    closed_at TIMESTAMPTZ,
    closed_by_user_id UUID,
    close_reason VARCHAR(1000),
    deleted_at TIMESTAMPTZ,
    deleted_by_user_id UUID,
    idempotency_key VARCHAR(100),
    request_hash CHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT workspace_tasks_project_fk FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_tasks_assignee_fk FOREIGN KEY (assignee_member_id, project_id)
        REFERENCES project_members(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_tasks_creator_fk FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_tasks_updater_fk FOREIGN KEY (updated_by_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_tasks_completed_by_fk FOREIGN KEY (completed_by_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_tasks_closed_by_fk FOREIGN KEY (closed_by_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_tasks_deleted_by_fk FOREIGN KEY (deleted_by_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_tasks_id_project_unique UNIQUE (id, project_id),
    CONSTRAINT workspace_tasks_status_check CHECK (status IN ('todo', 'in_progress', 'review', 'done', 'closed')),
    CONSTRAINT workspace_tasks_priority_check CHECK (priority IN ('low', 'medium', 'high')),
    CONSTRAINT workspace_tasks_version_check CHECK (version > 0 AND content_revision > 0),
    CONSTRAINT workspace_tasks_idempotency_shape_check CHECK (
        (idempotency_key IS NULL AND request_hash IS NULL) OR
        (idempotency_key IS NOT NULL AND request_hash IS NOT NULL)
    ),
    CONSTRAINT workspace_tasks_close_shape_check CHECK (
        (status = 'closed' AND closed_at IS NOT NULL AND closed_by_user_id IS NOT NULL AND close_reason IS NOT NULL) OR
        (status <> 'closed')
    )
);

CREATE INDEX workspace_tasks_board_idx ON workspace_tasks(project_id, status, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX workspace_tasks_assignee_status_idx ON workspace_tasks(project_id, assignee_member_id, status) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX workspace_tasks_idempotency_idx ON workspace_tasks(project_id, created_by_user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE workspace_task_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    task_id UUID NOT NULL,
    revision_number BIGINT NOT NULL,
    title VARCHAR(160) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    project_context_snapshot_id UUID NOT NULL,
    changed_by_user_id UUID NOT NULL,
    changed_by_member_id UUID,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT workspace_task_revisions_task_fk FOREIGN KEY (task_id, project_id)
        REFERENCES workspace_tasks(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_revisions_context_fk FOREIGN KEY (project_context_snapshot_id, project_id)
        REFERENCES workspace_project_context_snapshots(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_revisions_actor_fk FOREIGN KEY (changed_by_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_revisions_member_fk FOREIGN KEY (changed_by_member_id, project_id)
        REFERENCES project_members(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_revisions_number_unique UNIQUE (task_id, revision_number),
    CONSTRAINT workspace_task_revisions_id_task_project_unique UNIQUE (id, task_id, project_id),
    CONSTRAINT workspace_task_revisions_number_check CHECK (revision_number > 0)
);

CREATE TABLE workspace_task_review_cycles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    task_id UUID NOT NULL,
    cycle_number BIGINT NOT NULL,
    task_revision_id UUID NOT NULL,
    project_context_snapshot_id UUID NOT NULL,
    assignee_member_id_at_request UUID NOT NULL,
    assignee_role_id_at_request UUID,
    assignee_role_name_at_request VARCHAR(160),
    requested_by_user_id UUID NOT NULL,
    requested_by_member_id UUID,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    state VARCHAR(24) NOT NULL DEFAULT 'pending',
    resolved_by_user_id UUID,
    resolved_by_member_id UUID,
    resolved_at TIMESTAMPTZ,
    resolution_reason VARCHAR(1000),
    reviewer_is_assignee BOOLEAN,
    reviewer_is_requester BOOLEAN,
    CONSTRAINT workspace_task_cycles_task_fk FOREIGN KEY (task_id, project_id)
        REFERENCES workspace_tasks(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_cycles_revision_fk FOREIGN KEY (task_revision_id, task_id, project_id)
        REFERENCES workspace_task_revisions(id, task_id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_cycles_context_fk FOREIGN KEY (project_context_snapshot_id, project_id)
        REFERENCES workspace_project_context_snapshots(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_cycles_assignee_fk FOREIGN KEY (assignee_member_id_at_request, project_id)
        REFERENCES project_members(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_cycles_requester_fk FOREIGN KEY (requested_by_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_cycles_requester_member_fk FOREIGN KEY (requested_by_member_id, project_id)
        REFERENCES project_members(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_cycles_resolver_fk FOREIGN KEY (resolved_by_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_cycles_resolver_member_fk FOREIGN KEY (resolved_by_member_id, project_id)
        REFERENCES project_members(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_cycles_number_unique UNIQUE (task_id, cycle_number),
    CONSTRAINT workspace_task_cycles_id_task_project_unique UNIQUE (id, task_id, project_id),
    CONSTRAINT workspace_task_cycles_state_check CHECK (state IN ('pending', 'accepted', 'changes_requested', 'withdrawn', 'superseded')),
    CONSTRAINT workspace_task_cycles_resolution_shape_check CHECK (
        (state = 'pending' AND resolved_at IS NULL AND resolved_by_user_id IS NULL) OR
        (state <> 'pending' AND resolved_at IS NOT NULL AND resolved_by_user_id IS NOT NULL)
    ),
    CONSTRAINT workspace_task_cycles_self_review_shape_check CHECK (
        (reviewer_is_assignee IS NULL AND reviewer_is_requester IS NULL) OR
        (reviewer_is_assignee IS NOT NULL AND reviewer_is_requester IS NOT NULL)
    )
);
CREATE UNIQUE INDEX workspace_task_cycles_pending_idx ON workspace_task_review_cycles(task_id) WHERE state = 'pending';

CREATE TABLE workspace_task_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    task_id UUID NOT NULL,
    author_user_id UUID NOT NULL,
    author_member_id UUID,
    body VARCHAR(2000) NOT NULL,
    task_revision_id UUID NOT NULL,
    review_cycle_id_at_creation UUID,
    idempotency_key VARCHAR(100),
    request_hash CHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT workspace_task_comments_task_fk FOREIGN KEY (task_id, project_id)
        REFERENCES workspace_tasks(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_comments_author_fk FOREIGN KEY (author_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_comments_member_fk FOREIGN KEY (author_member_id, project_id)
        REFERENCES project_members(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_comments_revision_fk FOREIGN KEY (task_revision_id, task_id, project_id)
        REFERENCES workspace_task_revisions(id, task_id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_comments_cycle_fk FOREIGN KEY (review_cycle_id_at_creation, task_id, project_id)
        REFERENCES workspace_task_review_cycles(id, task_id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_task_comments_idempotency_shape_check CHECK (
        (idempotency_key IS NULL AND request_hash IS NULL) OR
        (idempotency_key IS NOT NULL AND request_hash IS NOT NULL)
    )
);
CREATE INDEX workspace_task_comments_order_idx ON workspace_task_comments(task_id, created_at, id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX workspace_task_comments_idempotency_idx ON workspace_task_comments(project_id, author_user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE workspace_activity_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    subject_type VARCHAR(32) NOT NULL,
    subject_id UUID NOT NULL,
    task_id UUID,
    project_context_snapshot_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    event_order SMALLINT NOT NULL,
    event_type VARCHAR(48) NOT NULL,
    actor_user_id UUID NOT NULL,
    actor_member_id UUID,
    actor_scope VARCHAR(32) NOT NULL,
    actor_role_id UUID,
    actor_role_name VARCHAR(160),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    schema_version SMALLINT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT workspace_activity_events_project_fk FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_activity_events_task_fk FOREIGN KEY (task_id, project_id)
        REFERENCES workspace_tasks(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_activity_events_context_fk FOREIGN KEY (project_context_snapshot_id, project_id)
        REFERENCES workspace_project_context_snapshots(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_activity_events_actor_fk FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_activity_events_member_fk FOREIGN KEY (actor_member_id, project_id)
        REFERENCES project_members(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_activity_events_operation_unique UNIQUE (operation_id, event_order),
    CONSTRAINT workspace_activity_events_order_check CHECK (event_order >= 1)
);
CREATE INDEX workspace_activity_events_project_order_idx ON workspace_activity_events(project_id, occurred_at DESC, id DESC);
CREATE INDEX workspace_activity_events_task_order_idx ON workspace_activity_events(task_id, occurred_at DESC, id DESC) WHERE task_id IS NOT NULL;
