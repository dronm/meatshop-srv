BEGIN;

INSERT INTO public.application_routes (
	name,
	path,
	descr,
	section,
	icon,
	menu_available,
	is_active
)
VALUES (
	'orderLines',
	'/order-lines',
	'Строки заказов',
	'Документы',
	'pi pi-list',
	true,
	true
)
ON CONFLICT (name) DO UPDATE
SET
	path = EXCLUDED.path,
	descr = EXCLUDED.descr,
	section = EXCLUDED.section,
	icon = EXCLUDED.icon,
	menu_available = EXCLUDED.menu_available,
	is_active = true;

INSERT INTO public.main_menus (
	role_id,
	parent_id,
	caption,
	route_id,
	icon,
	sort_order,
	is_active
)
SELECT
	'admin'::public.role_types,
	parent.id,
	'Строки заказов',
	route.id,
	NULL,
	20,
	true
FROM public.application_routes AS route
JOIN LATERAL (
	SELECT id
	FROM public.main_menus
	WHERE role_id = 'admin'::public.role_types
		AND user_id IS NULL
		AND parent_id IS NULL
		AND caption = 'Заказы'
	ORDER BY id
	LIMIT 1
) AS parent ON true
WHERE route.name = 'orderLines'
	AND NOT EXISTS (
		SELECT 1
		FROM public.main_menus AS existing
		WHERE existing.role_id = 'admin'::public.role_types
			AND existing.user_id IS NULL
			AND existing.route_id = route.id
	);

COMMIT;
