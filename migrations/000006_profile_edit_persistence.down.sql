DROP TABLE IF EXISTS career_histories;
DROP INDEX IF EXISTS profiles_village_id_idx;
ALTER TABLE profiles
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS village_id;
