CREATE TABLE workspace_discussions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    author_id UUID NOT NULL,
    title VARCHAR(160) NOT NULL,
    body TEXT NOT NULL,
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    idempotency_key VARCHAR(100),
    request_hash CHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT workspace_discussions_project_fk
        FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_discussions_author_fk
        FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_discussions_id_project_unique UNIQUE (id, project_id),
    CONSTRAINT workspace_discussions_idempotency_shape_check CHECK (
        (idempotency_key IS NULL AND request_hash IS NULL) OR
        (idempotency_key IS NOT NULL AND request_hash IS NOT NULL)
    )
);

CREATE INDEX workspace_discussions_project_activity_idx
    ON workspace_discussions(project_id, last_activity_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX workspace_discussions_idempotency_idx
    ON workspace_discussions(project_id, author_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE workspace_discussion_replies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    discussion_id UUID NOT NULL,
    parent_reply_id UUID,
    author_id UUID NOT NULL,
    body VARCHAR(2000) NOT NULL,
    idempotency_key VARCHAR(100),
    request_hash CHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT workspace_discussion_replies_project_fk
        FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_discussion_replies_discussion_fk
        FOREIGN KEY (discussion_id, project_id)
        REFERENCES workspace_discussions(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_discussion_replies_parent_fk
        FOREIGN KEY (parent_reply_id, discussion_id, project_id)
        REFERENCES workspace_discussion_replies(id, discussion_id, project_id) ON DELETE RESTRICT,
    CONSTRAINT workspace_discussion_replies_author_fk
        FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT workspace_discussion_replies_id_discussion_project_unique
        UNIQUE (id, discussion_id, project_id),
    CONSTRAINT workspace_discussion_replies_idempotency_shape_check CHECK (
        (idempotency_key IS NULL AND request_hash IS NULL) OR
        (idempotency_key IS NOT NULL AND request_hash IS NOT NULL)
    )
);

CREATE INDEX workspace_discussion_replies_thread_order_idx
    ON workspace_discussion_replies(discussion_id, parent_reply_id, created_at, id)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX workspace_discussion_replies_idempotency_idx
    ON workspace_discussion_replies(project_id, author_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
