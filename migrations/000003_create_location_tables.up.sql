CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 1. provinces
CREATE TABLE provinces (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(10) NOT NULL UNIQUE,
    name VARCHAR(150) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. regencies
CREATE TABLE regencies (
    id BIGSERIAL PRIMARY KEY,
    province_id BIGINT NOT NULL REFERENCES provinces(id) ON DELETE CASCADE,
    code VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(150) NOT NULL,
    type VARCHAR(20) NOT NULL, -- 'regency' or 'city'
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_regencies_province ON regencies(province_id);

-- 3. districts
CREATE TABLE districts (
    id BIGSERIAL PRIMARY KEY,
    regency_id BIGINT NOT NULL REFERENCES regencies(id) ON DELETE CASCADE,
    code VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(150) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_districts_regency ON districts(regency_id);

-- 4. villages
CREATE TABLE villages (
    id BIGSERIAL PRIMARY KEY,
    district_id BIGINT NOT NULL REFERENCES districts(id) ON DELETE CASCADE,
    code VARCHAR(20) NOT NULL UNIQUE,
    postal_code VARCHAR(10) NOT NULL,
    name VARCHAR(150) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_villages_district ON villages(district_id);

-- Trigram Indexes for Autocomplete
CREATE INDEX idx_villages_name_trgm ON villages USING gin (name gin_trgm_ops);
CREATE INDEX idx_districts_name_trgm ON districts USING gin (name gin_trgm_ops);
CREATE INDEX idx_regencies_name_trgm ON regencies USING gin (name gin_trgm_ops);
