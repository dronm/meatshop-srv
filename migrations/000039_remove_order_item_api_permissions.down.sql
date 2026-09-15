BEGIN;

WITH required(code, description) AS (
	VALUES
		('orderItem.list', 'Просмотр строк заказа'),
		('orderItem.detail', 'Просмотр строки заказа')
)
INSERT INTO public.permissions (code, description)
SELECT required.code, required.description
FROM required
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

WITH roles(role_id) AS (
	VALUES ('admin'::public.role_types)
), required(permission_code) AS (
	VALUES
		('orderItem.list'),
		('orderItem.detail')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT roles.role_id, required.permission_code
FROM roles
CROSS JOIN required
ON CONFLICT DO NOTHING;

COMMIT;
