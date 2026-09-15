-- Function: public.customers_ref(customers)

-- DROP FUNCTION public.customers_ref(customers);

CREATE OR REPLACE FUNCTION public.customers_ref(customers)
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
				'dataType','customers'
			)
	END;
$BODY$
  LANGUAGE sql VOLATILE
  COST 100;





