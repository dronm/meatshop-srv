BEGIN;

ALTER TABLE public.order_items
	ADD COLUMN use_marking bool NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN public.order_items.use_marking IS 'Line use marking flag.';

COMMIT;

