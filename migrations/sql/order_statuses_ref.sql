-- Function: public.order_statuses_ref(order_statuses)

-- DROP FUNCTION public.order_statuses_ref(order_statuses);

CREATE OR REPLACE FUNCTION public.order_statuses_ref(order_statuses)
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
				'dataType','order_statuses'
			)
	END;
$BODY$
  LANGUAGE sql VOLATILE
  COST 100;






