BEGIN;

LOCK TABLE public.orders IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM public.orders WHERE customer_sale_place_id IS NULL) THEN
		RAISE EXCEPTION 'Cannot roll back cash orders: assign a sale place to every order first.';
	END IF;
END;
$$;

DROP VIEW public.orders_list;
DROP VIEW public.order_lines_list;

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
	orders.shipment_ref_1c
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
	o.shipment_number_1c
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


ALTER TABLE public.orders
	DROP CONSTRAINT orders_sale_place_required_unless_cash_chk,
	ALTER COLUMN customer_sale_place_id SET NOT NULL,
	DROP COLUMN customer_phone,
	DROP COLUMN customer_name,
	DROP COLUMN cash_payed;

COMMENT ON COLUMN public.orders.customer_sale_place_id IS 'Customer sale place reference';

-- Keep editable notification template bodies; the condition remains valid.

COMMIT;
