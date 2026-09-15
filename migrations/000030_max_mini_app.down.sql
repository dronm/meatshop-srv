BEGIN;

DROP VIEW public.max_users_list;

CREATE VIEW public.max_users_list AS
SELECT
	u.id,
	CASE WHEN c.id IS NULL THEN NULL ELSE jsonb_build_object('keys', jsonb_build_object('id', c.id), 'descr', c.name) END AS customer,
	u.max_user_id,
	u.username,
	u.avatar_url,
	u.raw_user,
	u.is_active
FROM public.max_users u
LEFT JOIN public.customers c ON c.id = u.customer_id;

DROP INDEX IF EXISTS public.max_users_customer_sale_place_id_idx;

CREATE OR REPLACE VIEW public.customer_users_list AS
SELECT DISTINCT
	o.customer_id,
	u.id,
	u.max_user_id,
	u.username,
	u.avatar_url,
	u.is_active
FROM public.orders o
JOIN public.max_users u ON u.id = o.customer_user_id
WHERE o.customer_user_id IS NOT NULL;

UPDATE public.max_users
SET username = ''
WHERE username IS NULL;

ALTER TABLE public.max_users
	ALTER COLUMN username SET NOT NULL;

ALTER TABLE public.max_users
	DROP COLUMN IF EXISTS customer_sale_place_id;

COMMIT;
