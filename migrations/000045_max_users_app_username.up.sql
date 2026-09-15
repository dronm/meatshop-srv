BEGIN;

ALTER TABLE max_users ADD COLUMN IF NOT EXISTS app_username TEXT;
UPDATE max_users SET app_username = coalesce(username, 'Не задано') WHERE app_username IS NULL;
ALTER TABLE max_users ALTER COLUMN app_username SET NOT NULL;

COMMIT;
