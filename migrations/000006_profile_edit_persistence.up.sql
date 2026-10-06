ALTER TABLE profiles
    ADD COLUMN village_id BIGINT REFERENCES villages(id) ON DELETE SET NULL,
    ADD COLUMN address TEXT;

CREATE INDEX profiles_village_id_idx ON profiles (village_id);

CREATE TABLE career_histories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    title VARCHAR(150) NOT NULL CHECK (length(btrim(title)) >= 2),
    company VARCHAR(150) NOT NULL CHECK (length(btrim(company)) >= 2),
    start_year INTEGER NOT NULL CHECK (start_year >= 1950),
    end_year INTEGER CHECK (end_year IS NULL OR end_year >= start_year),
    description TEXT CHECK (description IS NULL OR length(description) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX career_histories_user_id_start_year_idx
    ON career_histories (user_id, start_year DESC, created_at DESC);
