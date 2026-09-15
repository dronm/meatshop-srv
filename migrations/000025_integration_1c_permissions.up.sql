BEGIN;

WITH required(code, description) AS (
	VALUES
		('integration1c.nomenclature.complete', 'Поиск номенклатуры в 1С'),
		('integration1c.counterparty.complete', 'Поиск контрагентов в 1С')
)
INSERT INTO public.permissions (code, description)
SELECT required.code, required.description
FROM required
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

WITH required(permission_code) AS (
	VALUES
		('integration1c.nomenclature.complete'),
		('integration1c.counterparty.complete')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT 'admin'::public.role_types, required.permission_code
FROM required
ON CONFLICT DO NOTHING;

COMMIT;
