BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code = 'integration1c.jobs.list';

DELETE FROM public.permissions
WHERE code = 'integration1c.jobs.list';

COMMIT;
