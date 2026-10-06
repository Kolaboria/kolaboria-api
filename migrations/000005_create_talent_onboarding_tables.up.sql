CREATE TABLE talent_profiles (
    user_id UUID PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
    experience_level VARCHAR(20) NOT NULL
        CHECK (experience_level IN ('Beginner', 'Intermediate', 'Advanced', 'Expert')),
    goal TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_skills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE RESTRICT,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_skills_user_skill_unique UNIQUE (user_id, skill_id)
);

CREATE UNIQUE INDEX user_skills_one_primary_per_user_idx
    ON user_skills (user_id)
    WHERE is_primary;

CREATE INDEX user_skills_skill_id_idx ON user_skills (skill_id);

CREATE TABLE user_tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    tool_id UUID NOT NULL REFERENCES tools(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_tools_user_tool_unique UNIQUE (user_id, tool_id)
);

CREATE INDEX user_tools_tool_id_idx ON user_tools (tool_id);
