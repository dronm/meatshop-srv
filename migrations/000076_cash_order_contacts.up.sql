BEGIN;

ALTER TABLE public.orders
	ADD COLUMN cash_payed boolean NOT NULL DEFAULT false,
	ADD COLUMN customer_name text,
	ADD COLUMN customer_phone text,
	ALTER COLUMN customer_sale_place_id DROP NOT NULL,
	ADD CONSTRAINT orders_sale_place_required_unless_cash_chk
		CHECK (cash_payed OR customer_sale_place_id IS NOT NULL);

COMMENT ON COLUMN public.orders.cash_payed IS
	'Cash order with optional buyer contact details and no sale place.';
COMMENT ON COLUMN public.orders.customer_name IS
	'Optional actual buyer name saved with the order.';
COMMENT ON COLUMN public.orders.customer_phone IS
	'Optional actual buyer telephone saved with the order.';
COMMENT ON COLUMN public.orders.customer_sale_place_id IS
	'Customer sale place reference; required unless cash_payed is true.';

-- View: public.orders_list

-- DROP VIEW public.orders_list;

CREATE OR REPLACE VIEW public.orders_list AS
SELECT
	orders.id,
	orders.number_1c,
	orders.for_date,
	orders.ref_1c,
	orders.customer_id,
	customers_ref(customer) AS customer,
	customer_sale_places_ref(sale_place) AS customer_sale_place,
	customer_users_ref(customer_user) AS customer_user,
	order_statuses_ref(order_status) AS status,
	orders.comment_customer,
	orders.comment_admin,
	orders.shipment_ref_1c,
	orders.cash_payed,
	orders.customer_name,
	orders.customer_phone
FROM public.orders AS orders
LEFT JOIN public.customers AS customer
	ON customer.id = orders.customer_id
LEFT JOIN public.customer_sale_places AS sale_place
	ON sale_place.id = orders.customer_sale_place_id
LEFT JOIN public.max_users AS customer_user
	ON customer_user.id = orders.customer_user_id
LEFT JOIN public.order_statuses AS order_status
	ON order_status.id = orders.status_id;

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
	items.weight_confirmed,
	items.mercury,
	o.number_1c,
	o.shipment_number_1c,
	o.cash_payed,
	o.customer_name,
	o.customer_phone
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


-- Omit the standard sale-place line from notifications when no place exists.
-- Preserve customized templates that no longer contain that exact line.
UPDATE public.notification_templates
SET body_template = replace(
	body_template,
	E'\nТочка продажи: {{.SalePlaceName}}',
	E'{{if .SalePlaceName}}\nТочка продажи: {{.SalePlaceName}}{{end}}'
)
WHERE event IN ('order.changed', 'order.submitted')
	AND strpos(body_template, '{{if .SalePlaceName}}') = 0;

COMMIT;
