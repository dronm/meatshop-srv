BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'integration1c.nomenclature.complete',
	'integration1c.counterparty.complete'
);

DELETE FROM public.permissions
WHERE code IN (
	'integration1c.nomenclature.complete',
	'integration1c.counterparty.complete'
);

COMMIT;
