BEGIN;

ALTER TABLE public.orders
	ADD COLUMN version bigint NOT NULL DEFAULT 1;

COMMENT ON COLUMN public.orders.version IS 'Optimistic concurrency version incremented after every complete order update.';

COMMIT;
