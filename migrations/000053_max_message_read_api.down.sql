BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'maxOutMessage.list',
	'maxOutMessage.detail',
	'maxInMessage.list',
	'maxInMessage.detail'
);

DELETE FROM public.permissions
WHERE code IN (
	'maxOutMessage.list',
	'maxOutMessage.detail',
	'maxInMessage.list',
	'maxInMessage.detail'
);

COMMIT;
