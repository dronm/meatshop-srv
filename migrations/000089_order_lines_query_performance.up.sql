BEGIN;

-- Pure reference builders depend only on their composite row argument.
-- STABLE lets COUNT discard unused JSON projections instead of evaluating
-- them for every order line.
ALTER FUNCTION public.customers_ref(public.customers) STABLE;
ALTER FUNCTION public.customer_sale_places_ref(public.customer_sale_places) STABLE;
ALTER FUNCTION public.products_ref(public.products) STABLE;
ALTER FUNCTION public.order_statuses_ref(public.order_statuses) STABLE;

-- Every item has a non-null order FK, so this inner join preserves rows and
-- allows PostgreSQL to start from the newest orders for a limited page.
-- The unique production order index makes the optional link join one-to-one.
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
	o.customer_phone,
	link.transfer_id AS production_transfer_id
FROM public.order_items AS items
JOIN public.orders AS o ON o.id = items.order_id
LEFT JOIN public.customers AS customer
	ON customer.id = o.customer_id
LEFT JOIN public.customer_sale_places AS sale_place
	ON sale_place.id = o.customer_sale_place_id
LEFT JOIN public.products AS p
	ON p.id = items.product_id
LEFT JOIN public.order_statuses AS order_status
	ON order_status.id = o.status_id
LEFT JOIN public.production_transfer_orders AS link
	ON link.order_id = o.id
WHERE items.quant > 0
ORDER BY o.id DESC, items.line_num, items.id;

COMMIT;
