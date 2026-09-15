BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'order.createShipments1c',
	'order.printShipment1c'
);

DELETE FROM public.permissions
WHERE code IN (
	'order.createShipments1c',
	'order.printShipment1c'
);

DROP INDEX IF EXISTS integration_1c.integration_1c_jobs_create_shipments_active_idx;

COMMIT;
