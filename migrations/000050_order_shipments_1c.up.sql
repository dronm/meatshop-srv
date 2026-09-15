BEGIN;

CREATE UNIQUE INDEX integration_1c_jobs_create_shipments_active_idx
	ON integration_1c.jobs (command, correlation_id)
	WHERE command = 'create_shipments'
		AND correlation_id IS NOT NULL
		AND status IN ('queued', 'processing');

INSERT INTO public.permissions (code, description)
VALUES
	('order.createShipments1c', 'Создание реализаций по заказам в 1С'),
	('order.printShipment1c', 'Печать реализаций из 1С')
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

WITH required(permission_code) AS (
	VALUES
		('order.createShipments1c'),
		('order.printShipment1c')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT 'admin'::public.role_types, required.permission_code
FROM required
ON CONFLICT DO NOTHING;

COMMIT;
