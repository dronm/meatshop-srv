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
