-- Function: public.products_ref(products)

-- DROP FUNCTION public.products_ref(products);

CREATE OR REPLACE FUNCTION public.products_ref(products)
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
				'dataType','products'
			)
	END;
$BODY$
  LANGUAGE sql VOLATILE
  COST 100;






