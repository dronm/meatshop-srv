BEGIN;

WITH required(code, description) AS (
	VALUES
		('maxOutMessage.list', 'Просмотр исходящих сообщений MAX'),
		('maxOutMessage.detail', 'Просмотр исходящего сообщения MAX'),
		('maxInMessage.list', 'Просмотр входящих сообщений MAX'),
		('maxInMessage.detail', 'Просмотр входящего сообщения MAX')
)
INSERT INTO public.permissions (code, description)
SELECT required.code, required.description
FROM required
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

WITH required(permission_code) AS (
	VALUES
		('maxOutMessage.list'),
		('maxOutMessage.detail'),
		('maxInMessage.list'),
		('maxInMessage.detail')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT 'admin'::public.role_types, required.permission_code
FROM required
ON CONFLICT DO NOTHING;

COMMIT;
