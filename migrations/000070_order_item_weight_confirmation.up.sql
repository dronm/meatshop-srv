BEGIN;

ALTER TABLE public.products
	ADD COLUMN requires_weight_confirmation boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN public.products.requires_weight_confirmation IS
	'Requires explicit confirmation of the actual weight on new order items.';

-- Existing orders have already passed the old workflow. New item writes set
-- their initial state from the product policy rather than retroactively marking
-- historical quantities for review.
ALTER TABLE public.order_items
	ADD COLUMN weight_confirmed boolean NOT NULL DEFAULT true;

ALTER TABLE public.order_items
	ALTER COLUMN weight_confirmed SET DEFAULT false;

COMMENT ON COLUMN public.order_items.weight_confirmed IS
	'Actual weight was explicitly confirmed for this order item.';

INSERT INTO public.audit_column_aliases (
	table_name, column_name, column_alias, is_active
)
VALUES
	('products', 'requires_weight_confirmation', 'Требует проверки веса', true),
	('order_items', 'weight_confirmed', 'Вес проверен', true)
ON CONFLICT (table_name, column_name) DO UPDATE
SET column_alias = EXCLUDED.column_alias,
	is_active = true;

-- View: public.products_list

-- DROP VIEW public.products_list;

CREATE OR REPLACE VIEW public.products_list
 AS
SELECT 
	t.id,
	t.parent_id,
	t.name,
	t.measure_unit_id,
	measure_units_ref(mu) AS measure_unit,
	t.sort_order,
	t.is_group,
	t.is_active,
	t.ref_1c,
	t.requires_weight_confirmation

FROM public.products AS t
LEFT JOIN measure_units AS mu ON mu.id = t.measure_unit_id
;

-- View: public.order_lines_list

-- DROP VIEW public.order_lines_list;

CREATE OR REPLACE VIEW public.order_lines_list AS
SELECT
	o.id,
	o.shipment_ref_1c,
	customers_ref(customer) AS customer,
	customer_sale_places_ref(sale_place) AS customer_sale_place,
	products_ref(p) AS product,
	items.quant,
	items.price,
	items.amount,
	items.vat_amount,
	COALESCE(customer.use_marking, false) AS use_marking,
	items.id AS item_id,
	items.line_num,
	o.ref_1c,
	o.for_date,
	order_statuses_ref(order_status) AS status,
	o.comment_customer,
	o.comment_admin,
	COALESCE(p.requires_weight_confirmation, false) AS requires_weight_confirmation,
	items.weight_confirmed
FROM public.order_items AS items
LEFT JOIN public.orders AS o ON o.id = items.order_id
LEFT JOIN public.customers AS customer
	ON customer.id = o.customer_id
LEFT JOIN public.customer_sale_places AS sale_place
	ON sale_place.id = o.customer_sale_place_id
LEFT JOIN public.products AS p
	ON p.id = items.product_id
LEFT JOIN public.order_statuses AS order_status
	ON order_status.id = o.status_id
WHERE items.quant > 0
ORDER BY o.id DESC, items.line_num, items.id;

COMMIT;
