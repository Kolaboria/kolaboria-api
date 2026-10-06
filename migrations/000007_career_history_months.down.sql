DROP INDEX career_histories_user_id_start_year_idx;
CREATE INDEX career_histories_user_id_start_year_idx
    ON career_histories (user_id, start_year DESC, created_at DESC);

ALTER TABLE career_histories
    DROP CONSTRAINT career_histories_month_order_check,
    DROP CONSTRAINT career_histories_end_month_check,
    DROP CONSTRAINT career_histories_start_month_check,
    DROP COLUMN end_month,
    DROP COLUMN start_month;
