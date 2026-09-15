-- Function: public.users_ref(users)

-- DROP FUNCTION public.users_ref(orders);

CREATE OR REPLACE FUNCTION public.users_ref(users)
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
				'dataType','users'
			)
	END;
$BODY$
  LANGUAGE sql VOLATILE
  COST 100;







