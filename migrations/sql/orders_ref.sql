-- Function: public.orders_ref(orders)

-- DROP FUNCTION public.orders_ref(orders);

CREATE OR REPLACE FUNCTION public.orders_ref(orders)
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
				'descr',$1.number_1c,
				'dataType','orders'
			)
	END;
$BODY$
  LANGUAGE sql VOLATILE
  COST 100;






