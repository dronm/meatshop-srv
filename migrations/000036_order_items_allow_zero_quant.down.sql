BEGIN;

ALTER TABLE public.order_items
	DROP CONSTRAINT IF EXISTS order_items_quant_check;

ALTER TABLE public.order_items
	ADD CONSTRAINT order_items_quant_check CHECK (quant > 0);

COMMIT;
