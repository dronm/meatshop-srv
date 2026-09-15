BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'orderItem.list',
	'orderItem.detail'
);

DELETE FROM public.permissions
WHERE code IN (
	'orderItem.list',
	'orderItem.detail'
);

COMMIT;
