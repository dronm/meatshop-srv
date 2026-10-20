BEGIN;

CREATE UNIQUE INDEX integration_1c_jobs_create_orders_active_idx
	ON integration_1c.jobs (command, correlation_id)
	WHERE command = 'create_orders'
		AND correlation_id IS NOT NULL
		AND status IN ('queued', 'processing');

COMMIT;
