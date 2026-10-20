BEGIN;

DELETE FROM public.main_menus
WHERE route_id IN (SELECT id FROM public.application_routes WHERE name = 'integration1cAdmin');
DELETE FROM public.application_routes WHERE name = 'integration1cAdmin';
DELETE FROM public.role_permissions WHERE permission_code = 'integration1c.admin';
DELETE FROM public.permissions WHERE code = 'integration1c.admin';

COMMIT;
