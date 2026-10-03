-- 1. Enum Tipe Token
DO $$ BEGIN
    CREATE TYPE token_type AS ENUM (
        'refresh',
        'email_verification',
        'password_reset'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- 2. Tabel Users (Identitas & Kredensial Inti)
CREATE TABLE IF NOT EXISTS users (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email             VARCHAR(255) NOT NULL UNIQUE,
    password_hash     VARCHAR(255) NOT NULL,
    email_verified_at TIMESTAMPTZ,
    is_active         BOOLEAN NOT NULL DEFAULT true,
    google_id         VARCHAR(255) UNIQUE,
    provider          VARCHAR(20) NOT NULL DEFAULT 'email',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);

-- 3. Tabel Profiles (Field esensial)
CREATE TABLE IF NOT EXISTS profiles (
    id                  UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    username            VARCHAR(100) NOT NULL UNIQUE,
    full_name           VARCHAR(255),
    headline            TEXT,
    bio                 TEXT,
    avatar              TEXT,
    location            VARCHAR(255),
    account_type        VARCHAR(20) CHECK (account_type IN ('talent')),
    external_links      JSONB NOT NULL DEFAULT '{}',
    is_verified         BOOLEAN NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_profiles_username ON profiles(username);
CREATE INDEX IF NOT EXISTS idx_profiles_account_type ON profiles(account_type);

-- 4. Tabel User Tokens (Refresh, Verifikasi Email, Reset Password)
CREATE TABLE IF NOT EXISTS user_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       TEXT NOT NULL UNIQUE,
    type        token_type NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_tokens_token ON user_tokens(token);
CREATE INDEX IF NOT EXISTS idx_user_tokens_user_id_type ON user_tokens(user_id, type);
