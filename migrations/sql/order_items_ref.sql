-- Function: public.order_itmes_ref(order_itmes)

 --DROP FUNCTION public.order_items_ref(order_itmes);

CREATE OR REPLACE FUNCTION public.order_items_ref(order_items)
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
				'descr',(SELECT p.name FROM products p WHERE p.id = $1.product_id),
				'dataType','order_items'
			)
	END;
$BODY$
  LANGUAGE sql VOLATILE
  COST 100;






