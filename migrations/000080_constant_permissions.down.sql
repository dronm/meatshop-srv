BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code IN ('constant.list', 'constant.detail', 'constant.get', 'constant.set');

DELETE FROM public.permissions
WHERE code IN ('constant.list', 'constant.detail', 'constant.get', 'constant.set');

COMMIT;
