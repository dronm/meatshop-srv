BEGIN;

INSERT INTO public.permissions (code, description)
VALUES ('integration1c.jobs.list', 'Просмотр очереди интеграции с 1С')
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

INSERT INTO public.role_permissions (role_id, permission_code)
VALUES ('admin'::public.role_types, 'integration1c.jobs.list')
ON CONFLICT DO NOTHING;

COMMIT;
