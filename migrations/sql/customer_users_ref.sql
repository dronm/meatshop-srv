-- Function: public.customer_users_ref(max_users)

-- DROP FUNCTION public.customer_users_ref(max_users);

CREATE OR REPLACE FUNCTION public.customer_users_ref(max_users)
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
				'descr',coalesce(
					nullif(btrim($1.app_username), ''),
					nullif(btrim($1.username), ''),
					$1.max_user_id::text
				),
				'dataType','customer_users'
			)
	END;
$BODY$
  LANGUAGE sql VOLATILE
  COST 100;





