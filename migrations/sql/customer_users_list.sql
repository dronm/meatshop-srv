-- View: public.customer_users_list

-- DROP VIEW public.customer_users_list;


CREATE OR REPLACE VIEW public.customer_users_list AS
SELECT DISTINCT
	o.customer_id,
	u.id,
	u.max_user_id,
	COALESCE(
		NULLIF(btrim(u.app_username), ''),
		NULLIF(btrim(u.username), ''),
		u.max_user_id::text
	) AS username,
	u.avatar_url,
	u.is_active
FROM public.orders o
JOIN public.max_users u ON u.id = o.customer_user_id
WHERE o.customer_user_id IS NOT NULL;

COMMENT ON VIEW public.customer_users_list IS
	'Deduplicated MAX users that have placed orders for a customer.';
