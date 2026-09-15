BEGIN;

CREATE UNIQUE INDEX integration_1c_jobs_create_order_active_idx
	ON integration_1c.jobs (command, correlation_id)
	WHERE command = 'create_order'
		AND correlation_id IS NOT NULL
		AND status IN ('queued', 'processing');

INSERT INTO public.permissions (code, description)
VALUES ('order.create1c', 'Создание заказа в 1С')
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

INSERT INTO public.role_permissions (role_id, permission_code)
VALUES ('admin'::public.role_types, 'order.create1c')
ON CONFLICT DO NOTHING;

COMMIT;
