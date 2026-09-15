BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code = 'order.print1c';

DELETE FROM public.permissions
WHERE code = 'order.print1c';

COMMIT;
