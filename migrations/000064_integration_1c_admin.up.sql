BEGIN;

INSERT INTO public.permissions (code, description)
VALUES ('integration1c.admin', 'Управление сервисом интеграции с 1С')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO public.role_permissions (role_id, permission_code)
VALUES ('admin'::public.role_types, 'integration1c.admin')
ON CONFLICT DO NOTHING;

INSERT INTO public.application_routes (name, path, descr, section, icon, menu_available, is_active)
VALUES ('integration1cAdmin', '/integration-1c/admin', 'Управление интеграцией 1С',
	'Администрирование', 'pi pi-server', true, true)
ON CONFLICT (name) DO UPDATE SET
	path = EXCLUDED.path,
	descr = EXCLUDED.descr,
	section = EXCLUDED.section,
	icon = EXCLUDED.icon,
	menu_available = EXCLUDED.menu_available,
	is_active = EXCLUDED.is_active;

INSERT INTO public.main_menus (role_id, parent_id, caption, route_id, icon, sort_order, is_active)
SELECT 'admin'::public.role_types,
	(SELECT id FROM public.main_menus
	 WHERE role_id = 'admin'::public.role_types AND user_id IS NULL
		AND parent_id IS NULL AND caption = 'Администрирование'
	 ORDER BY id LIMIT 1),
	'Управление интеграцией 1С', route.id, 'pi pi-server', 90, true
FROM public.application_routes AS route
WHERE route.name = 'integration1cAdmin'
	AND NOT EXISTS (
		SELECT 1 FROM public.main_menus AS existing
		WHERE existing.role_id = 'admin'::public.role_types
			AND existing.user_id IS NULL AND existing.route_id = route.id
	);

COMMIT;
