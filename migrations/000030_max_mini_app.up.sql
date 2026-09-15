BEGIN;

ALTER TABLE public.max_users
	ADD COLUMN customer_sale_place_id integer REFERENCES public.customer_sale_places(id) ON DELETE RESTRICT ON UPDATE CASCADE;

COMMENT ON COLUMN public.max_users.customer_sale_place_id IS 'Customer sale place selected during MAX registration.';

ALTER TABLE public.max_users
	ALTER COLUMN username DROP NOT NULL;

CREATE INDEX max_users_customer_sale_place_id_idx
	ON public.max_users (customer_sale_place_id);

DROP VIEW public.max_users_list;

CREATE VIEW public.max_users_list AS
SELECT
	u.id,
	CASE WHEN c.id IS NULL THEN NULL ELSE jsonb_build_object('keys', jsonb_build_object('id', c.id), 'descr', c.name) END AS customer,
	CASE WHEN sp.id IS NULL THEN NULL ELSE jsonb_build_object('keys', jsonb_build_object('id', sp.id), 'descr', sp.name) END AS customer_sale_place,
	u.max_user_id,
	u.username,
	u.avatar_url,
	u.raw_user,
	u.is_active
FROM public.max_users u
LEFT JOIN public.customers c ON c.id = u.customer_id
LEFT JOIN public.customer_sale_places sp ON sp.id = u.customer_sale_place_id;

CREATE OR REPLACE VIEW public.customer_users_list AS
SELECT DISTINCT
	o.customer_id,
	u.id,
	u.max_user_id,
	COALESCE(u.username, u.max_user_id::text) AS username,
	u.avatar_url,
	u.is_active
FROM public.orders o
JOIN public.max_users u ON u.id = o.customer_user_id
WHERE o.customer_user_id IS NOT NULL;

UPDATE public.order_statuses
SET name = 'Принят'
WHERE code = 'accepted' AND btrim(name) = '';

COMMIT;
