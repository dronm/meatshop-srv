BEGIN;

INSERT INTO public.permissions (code, description) VALUES
	('constant.list', 'Просмотр списка констант'),
	('constant.detail', 'Просмотр описания константы'),
	('constant.get', 'Чтение значений констант'),
	('constant.set', 'Изменение значений констант')
ON CONFLICT (code) DO NOTHING;

INSERT INTO public.role_permissions (role_id, permission_code)
SELECT 'admin', code FROM public.permissions
WHERE code IN ('constant.list', 'constant.detail', 'constant.get', 'constant.set')
ON CONFLICT DO NOTHING;

COMMIT;
