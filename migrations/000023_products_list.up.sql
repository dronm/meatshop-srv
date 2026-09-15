CREATE OR REPLACE FUNCTION public.measure_units_ref(measure_units)
  RETURNS json AS
$BODY$
	SELECT 
	CASE
		WHEN $1.id IS NULL THEN NULL
		ELSE
			json_build_object(
				'keys',json_build_object(
					'id',$1.id    
					),	
				'descr',$1.name,
				'dataType','measure_units'
			)
	END;
$BODY$
  LANGUAGE sql VOLATILE
  COST 100;


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
	t.is_active

FROM public.products AS t
LEFT JOIN measure_units AS mu ON mu.id = t.measure_unit_id
;




