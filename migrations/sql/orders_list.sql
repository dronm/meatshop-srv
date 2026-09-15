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
