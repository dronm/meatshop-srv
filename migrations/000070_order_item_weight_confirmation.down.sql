BEGIN;

-- PostgreSQL requires recreating views to remove appended projection columns.
DROP VIEW public.order_lines_list;
DROP VIEW public.products_list;

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
	t.ref_1c

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
	o.comment_admin
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

DELETE FROM public.audit_column_aliases
WHERE (table_name = 'products' AND column_name = 'requires_weight_confirmation')
	OR (table_name = 'order_items' AND column_name = 'weight_confirmed');

ALTER TABLE public.order_items DROP COLUMN weight_confirmed;
ALTER TABLE public.products DROP COLUMN requires_weight_confirmation;

COMMIT;
