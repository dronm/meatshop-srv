BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code = 'order.create1c';

DELETE FROM public.permissions
WHERE code = 'order.create1c';

DROP INDEX IF EXISTS integration_1c.integration_1c_jobs_create_order_active_idx;

COMMIT;
