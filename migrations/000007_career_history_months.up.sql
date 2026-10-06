-- Keep unknown months NULL for existing year-only career histories.
ALTER TABLE career_histories
    ADD COLUMN start_month SMALLINT,
    ADD COLUMN end_month SMALLINT,
    ADD CONSTRAINT career_histories_start_month_check
        CHECK (start_month IS NULL OR start_month BETWEEN 1 AND 12),
    ADD CONSTRAINT career_histories_end_month_check
        CHECK (end_month IS NULL OR (end_year IS NOT NULL AND end_month BETWEEN 1 AND 12)),
    ADD CONSTRAINT career_histories_month_order_check
        CHECK (end_year IS NULL OR end_year > start_year
            OR start_month IS NULL OR end_month IS NULL OR end_month >= start_month);

DROP INDEX career_histories_user_id_start_year_idx;
CREATE INDEX career_histories_user_id_start_year_idx
    ON career_histories (user_id, start_year DESC, start_month DESC NULLS LAST, created_at DESC);
