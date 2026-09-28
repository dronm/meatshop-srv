BEGIN;

DELETE FROM public.main_menus AS menu
USING public.application_routes AS route
WHERE menu.route_id = route.id
	AND menu.role_id = 'admin'::public.role_types
	AND menu.user_id IS NULL
	AND route.name = 'orderLines';

DELETE FROM public.application_routes AS route
WHERE route.name = 'orderLines'
	AND NOT EXISTS (
		SELECT 1
		FROM public.main_menus AS menu
		WHERE menu.route_id = route.id
	);

COMMIT;
