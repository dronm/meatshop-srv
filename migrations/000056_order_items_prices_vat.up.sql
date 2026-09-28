BEGIN;

ALTER TABLE public.order_items
	ADD COLUMN price numeric(19, 6),
	ADD COLUMN amount numeric(15, 2),
	ADD COLUMN vat_percent numeric(5, 2),
	ADD COLUMN vat_amount numeric(15, 2);

COMMENT ON COLUMN public.order_items.price IS 'Unit price.';
COMMENT ON COLUMN public.order_items.amount IS 'Line amount with VAT.';
COMMENT ON COLUMN public.order_items.vat_percent IS 'Line VAT percent.';
COMMENT ON COLUMN public.order_items.vat_amount IS 'Line VAT amount.';

COMMIT;
