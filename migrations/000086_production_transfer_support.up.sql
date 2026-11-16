BEGIN;

CREATE OR REPLACE FUNCTION public.production_format_quant(value numeric)
RETURNS text LANGUAGE sql IMMUTABLE STRICT AS $$
	SELECT replace(CASE WHEN strpos(value::text, '.') > 0
		THEN rtrim(rtrim(value::text, '0'), '.') ELSE value::text END, '.', ',');
$$;

CREATE OR REPLACE VIEW public.production_order_options AS
SELECT o.id, o.version, o.for_date, o.number_1c,
	json_build_object('keys', json_build_object('id', o.id), 'descr',
		COALESCE(NULLIF(o.number_1c, ''), '№' || o.id::text) || ' от ' || to_char(o.for_date, 'DD.MM.YY')) AS order_ref,
	json_build_object('keys', json_build_object('id', c.id), 'descr',
		CASE WHEN o.cash_payed THEN COALESCE(NULLIF(o.customer_name, ''), c.name) ELSE c.name END) AS customer,
	CASE WHEN p.id IS NULL THEN NULL ELSE customer_sale_places_ref(p) END AS customer_sale_place,
	COALESCE(products.descr, '') AS products,
	COALESCE(s.code, '') AS status_code,
	o.comment_customer, o.comment_admin
FROM public.orders o
JOIN public.customers c ON c.id = o.customer_id
LEFT JOIN public.customer_sale_places p ON p.id = o.customer_sale_place_id
LEFT JOIN public.order_statuses s ON s.id = o.status_id
LEFT JOIN LATERAL (
	SELECT string_agg(pr.name || ' — ' || production_format_quant(i.quant) || ' ' || u.name,
		', ' ORDER BY i.line_num, i.id) AS descr
	FROM public.order_items i
	JOIN public.products pr ON pr.id = i.product_id
	JOIN public.measure_units u ON u.id = i.measure_unit_id
	WHERE i.order_id = o.id AND i.quant > 0
) products ON true;

CREATE OR REPLACE VIEW public.production_transfer_orders_list AS
SELECT l.id, l.transfer_id, l.order_id, l.line_num, o.order_ref AS "order",
	o.for_date, o.number_1c, o.customer, o.customer_sale_place, o.products
FROM public.production_transfer_orders l
JOIN public.production_order_options o ON o.id = l.order_id;

CREATE OR REPLACE VIEW public.production_transfers_list AS
SELECT t.id, t.version, t.document_date, t.for_date, t.comment,
	COALESCE(totals.descr, '') AS product_totals,
	COALESCE(customers.descr, '') AS customers
FROM public.production_transfers t
LEFT JOIN LATERAL (
	SELECT string_agg(unit_name || E':\n' || product_lines || E'\nИтого: ' || production_format_quant(unit_total) || ' ' || unit_name,
		E'\n\n' ORDER BY CASE lower(unit_name) WHEN 'кг' THEN 0 WHEN 'кор' THEN 1 ELSE 2 END, unit_name) AS descr
	FROM (
		SELECT measure_unit_id, unit_name,
			string_agg(product_name || ' — ' || production_format_quant(quant) || ' ' || unit_name, E'\n' ORDER BY product_name, product_id) AS product_lines,
			sum(quant) AS unit_total
		FROM (
			SELECT i.product_id, i.measure_unit_id, p.name AS product_name, u.name AS unit_name, sum(i.quant) AS quant
			FROM public.production_transfer_orders l
			JOIN public.order_items i ON i.order_id = l.order_id AND i.quant > 0
			JOIN public.products p ON p.id = i.product_id
			JOIN public.measure_units u ON u.id = i.measure_unit_id
			WHERE l.transfer_id = t.id
			GROUP BY i.product_id, i.measure_unit_id, p.name, u.name
		) per_product GROUP BY measure_unit_id, unit_name
	) per_unit
) totals ON true
LEFT JOIN LATERAL (
	SELECT string_agg(descr, ', ' ORDER BY descr) AS descr FROM (
		SELECT DISTINCT (o.customer->>'descr') || CASE WHEN o.customer_sale_place IS NULL THEN ''
			ELSE ' (' || (o.customer_sale_place->>'descr') || ')' END AS descr
		FROM public.production_transfer_orders l
		JOIN public.production_order_options o ON o.id = l.order_id
		WHERE l.transfer_id = t.id
	) names
) customers ON true;

CREATE TRIGGER audit_log_production_transfers
AFTER INSERT OR UPDATE OR DELETE ON public.production_transfers
FOR EACH ROW EXECUTE FUNCTION public.audit_log_process();
CREATE TRIGGER audit_log_production_transfer_orders
AFTER INSERT OR UPDATE OR DELETE ON public.production_transfer_orders
FOR EACH ROW EXECUTE FUNCTION public.audit_log_process();

INSERT INTO public.audit_column_aliases(table_name,column_name,column_alias,is_active) VALUES
	('production_transfers','id','Номер',true),
	('production_transfers','document_date','Дата документа',true),
	('production_transfers','for_date','Дата заявки',true),
	('production_transfers','comment','Комментарий',true),
	('production_transfers','version','Версия',true),
	('production_transfer_orders','transfer_id','Передача в производство',true),
	('production_transfer_orders','order_id','Заказ',true),
	('production_transfer_orders','line_num','Номер строки',true)
ON CONFLICT(table_name,column_name) DO UPDATE SET column_alias=EXCLUDED.column_alias,is_active=true;

COMMIT;
