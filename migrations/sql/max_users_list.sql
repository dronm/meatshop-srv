-- View: public.max_users_list

CREATE OR REPLACE VIEW public.max_users_list AS
SELECT
	u.id,
	CASE WHEN c.id IS NULL THEN NULL ELSE jsonb_build_object('keys', jsonb_build_object('id', c.id), 'descr', c.name) END AS customer,
	CASE WHEN sp.id IS NULL THEN NULL ELSE jsonb_build_object('keys', jsonb_build_object('id', sp.id), 'descr', sp.name) END AS customer_sale_place,
	u.max_user_id,
	u.username,
	u.avatar_url,
	u.raw_user,
	u.is_active,
	u.app_username
FROM public.max_users u
LEFT JOIN public.customers c ON c.id = u.customer_id
LEFT JOIN public.customer_sale_places sp ON sp.id = u.customer_sale_place_id;
