CREATE INDEX workspace_activity_events_feed_order_idx
    ON workspace_activity_events(project_id, occurred_at DESC, operation_id DESC, event_order DESC, id DESC);
