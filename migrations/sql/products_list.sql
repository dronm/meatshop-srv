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



