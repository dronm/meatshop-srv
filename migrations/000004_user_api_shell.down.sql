BEGIN;

DELETE FROM role_permissions
WHERE permission_code IN (
	'user.create',
	'mainMenu.create',
	'mainMenu.list',
	'mainMenu.for_user',
	'mainMenu.routeAutocomplete',
	'mainMenu.detail',
	'mainMenu.update',
	'mainMenu.delete',
	'applicationRoute.list',
	'applicationRoute.detail',
	'applicationRoute.update',
	'applicationRoute.sync',
	'progAbout.info'
);

DELETE FROM permissions
WHERE code IN (
	'user.create',
	'mainMenu.create',
	'mainMenu.list',
	'mainMenu.for_user',
	'mainMenu.routeAutocomplete',
	'mainMenu.detail',
	'mainMenu.update',
	'mainMenu.delete',
	'applicationRoute.list',
	'applicationRoute.detail',
	'applicationRoute.update',
	'applicationRoute.sync',
	'progAbout.info'
);

DROP VIEW IF EXISTS public.main_menus_list;
DROP TABLE IF EXISTS public.main_menus;
DROP TABLE IF EXISTS public.application_routes;

COMMIT;
