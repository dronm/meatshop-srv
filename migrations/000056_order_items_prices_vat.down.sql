BEGIN;

ALTER TABLE public.order_items
	DROP COLUMN vat_amount,
	DROP COLUMN vat_percent,
	DROP COLUMN amount,
	DROP COLUMN price;

COMMIT;
