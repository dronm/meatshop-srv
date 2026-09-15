-- View: public.audit_log_list

-- DROP VIEW public.audit_log_dialog;
-- DROP VIEW public.audit_log_list

CREATE OR REPLACE VIEW public.audit_log_list
 AS
SELECT 
	t.id,
	t.table_name,
	t.record_id,
	t.operation,
	t.changed_at,
	t.changed_by,
	CASE
		WHEN t.table_name = 'users' THEN users_ref(u)
		WHEN t.table_name = 'customers' THEN customers_ref(cust)
		WHEN t.table_name = 'customer_sale_places' THEN customer_sale_places_ref(cust_sp)
		WHEN t.table_name = 'max_users' THEN max_users_ref(max_u)
		WHEN t.table_name = 'measure_units' THEN measure_units_ref(mu)
		WHEN t.table_name = 'products' THEN products_ref(prod)
		WHEN t.table_name = 'orders' THEN orders_ref(orders)
		WHEN t.table_name = 'order_items' THEN order_items_ref(order_items)
		WHEN t.table_name = 'order_statuses' THEN order_statuses_ref(order_stat)
	ELSE NULL
	END AS object_ref,

	CASE
		WHEN t.table_name = 'users' THEN 'Пользователи'
		WHEN t.table_name = 'customers' THEN 'Покупатели'
		WHEN t.table_name = 'customer_sale_places' THEN 'Точки продаж'
		WHEN t.table_name = 'max_users' THEN 'Пользователи MAX'
		WHEN t.table_name = 'measure_units' THEN 'Единицы'
		WHEN t.table_name = 'products' THEN 'Номенклатура'
		WHEN t.table_name = 'orders' THEN 'Заказы'
		WHEN t.table_name = 'order_items' THEN 'Номенклатура заказов'
		WHEN t.table_name = 'order_statuses' THEN 'Статусы заказов'
	ELSE NULL
	END AS type_descr,

	CASE
		WHEN t.operation = 'I' THEN 'Добавление'
		WHEN t.operation = 'U' THEN 'Изменение'
		WHEN t.operation = 'D' THEN 'Удаление'
	ELSE NULL
	END AS operation_descr

FROM public.audit_log AS t
LEFT JOIN users AS u ON t.table_name = 'users' AND t.record_id = u.id::text
LEFT JOIN customers AS cust ON t.table_name = 'customers' AND t.record_id = cust.id::text
LEFT JOIN customer_sale_places AS cust_sp ON t.table_name = 'customer_sale_places' AND t.record_id = cust_sp.id::text
LEFT JOIN max_users AS max_u ON t.table_name = 'max_users' AND t.record_id = max_u.id::text
LEFT JOIN measure_units AS mu ON t.table_name = 'measure_units' AND t.record_id = mu.id::text
LEFT JOIN products AS prod ON t.table_name = 'products' AND t.record_id = prod.id::text
LEFT JOIN orders ON t.table_name = 'orders' AND t.record_id = orders.id::text
LEFT JOIN order_items ON t.table_name = 'order_items' AND t.record_id = order_items.id::text
LEFT JOIN order_statuses AS order_stat ON t.table_name = 'order_statuses' AND t.record_id = order_stat.id::text

ORDER BY t.changed_at DESC;


