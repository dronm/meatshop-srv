BEGIN;

INSERT INTO public.permissions (code, description)
VALUES ('order.print1c', 'Печать заказа из 1С')
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

INSERT INTO public.role_permissions (role_id, permission_code)
VALUES ('admin'::public.role_types, 'order.print1c')
ON CONFLICT DO NOTHING;

COMMIT;
