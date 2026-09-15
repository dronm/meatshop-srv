BEGIN;

CREATE OR REPLACE VIEW public.customer_users_list AS
SELECT DISTINCT
	o.customer_id,
	u.id,
	u.max_user_id,
	u.username,
	u.avatar_url,
	u.is_active
FROM public.orders AS o
JOIN public.max_users AS u ON u.id = o.customer_user_id
WHERE o.customer_user_id IS NOT NULL;

COMMENT ON VIEW public.customer_users_list IS
	'Deduplicated MAX users that have placed orders for a customer.';

COMMIT;
