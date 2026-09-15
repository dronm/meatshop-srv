-- View: public.users_list

 DROP VIEW public.users_list;

CREATE OR REPLACE VIEW public.users_list
 AS
SELECT 
	t.id,
	t.name,
	t.role_id,
	max_users_ref(mu) AS max_user

FROM public.users AS t
LEFT JOIN max_users AS mu ON mu.id = t.max_user_id
;

