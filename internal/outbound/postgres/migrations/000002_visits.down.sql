DROP INDEX IF EXISTS idx_visits_short_code;
DROP TABLE IF EXISTS visits;
ALTER TABLE links DROP COLUMN IF EXISTS click_count;