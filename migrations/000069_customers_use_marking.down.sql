BEGIN;

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
	items.use_marking,
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

-- Removing an appended view column requires recreating the projection.
DROP VIEW public.customers_list;

-- View: public.customers_list

-- DROP VIEW public.customers_list;

CREATE OR REPLACE VIEW public.customers_list
 AS
SELECT
	m.id,
	m.name,
	m.inn,
	m.kpp,
	NULLIF(btrim(m.ref_1c->>'id'), '') IS NOT NULL AS ref_1c_exists,
	m.is_active,
	m.allow_holiday_orders
FROM public.customers AS m
;

DELETE FROM public.audit_column_aliases
WHERE table_name = 'customers' AND column_name = 'use_marking';

ALTER TABLE public.customers DROP COLUMN use_marking;

COMMIT;
