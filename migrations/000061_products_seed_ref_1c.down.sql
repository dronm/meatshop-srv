-- Deliberately preserve seeded 1C references on downgrade.
-- Up records no before-images and performs no inserts. Clearing references
-- by UUID would also erase links that already existed before this migration.
-- Restore a database backup if the data itself must be reverted.
-- The migration version can be downgraded; the links remain intact.
BEGIN;
COMMIT;
