-- Function: public.customer_sale_places_ref(customer_sale_places)

-- DROP FUNCTION public.customer_sale_places_ref(customer_sale_places);

CREATE OR REPLACE FUNCTION public.customer_sale_places_ref(customer_sale_places)
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
				'dataType','customer_sale_places'
			)
	END;
$BODY$
  LANGUAGE sql VOLATILE
  COST 100;






