BEGIN;

DROP INDEX IF EXISTS public.customers_inn_kpp_idx;

CREATE UNIQUE INDEX customers_inn_idx
	ON public.customers USING btree (inn);

UPDATE public.max_users
SET app_username = COALESCE(
	NULLIF(btrim(app_username), ''),
	NULLIF(btrim(username), ''),
	'Не задано'
)
WHERE app_username IS NULL
	OR app_username <> btrim(app_username)
	OR btrim(app_username) = '';

ALTER TABLE public.max_users
	ADD CONSTRAINT max_users_app_username_not_blank_chk
	CHECK (btrim(app_username) <> '');

COMMENT ON COLUMN public.max_users.username IS
	'Username supplied by the MAX platform; may be unavailable.';
COMMENT ON COLUMN public.max_users.app_username IS
	'Application display name shown for the customer user.';

CREATE OR REPLACE FUNCTION public.max_users_ref(public.max_users)
	RETURNS json AS
$BODY$
	SELECT
	CASE
		WHEN $1.id IS NULL THEN NULL
		ELSE
			json_build_object(
				'keys', json_build_object(
					'id', $1.id
				),
				'descr', coalesce(
					nullif(btrim($1.app_username), ''),
					nullif(btrim($1.username), ''),
					$1.max_user_id::text
				),
				'dataType', 'max_users'
			)
	END;
$BODY$
	LANGUAGE sql VOLATILE
	COST 100;

CREATE OR REPLACE FUNCTION public.customer_users_ref(public.max_users)
	RETURNS json AS
$BODY$
	SELECT
	CASE
		WHEN $1.id IS NULL THEN NULL
		ELSE
			json_build_object(
				'keys', json_build_object(
					'id', $1.id
				),
				'descr', coalesce(
					nullif(btrim($1.app_username), ''),
					nullif(btrim($1.username), ''),
					$1.max_user_id::text
				),
				'dataType', 'customer_users'
			)
	END;
$BODY$
	LANGUAGE sql VOLATILE
	COST 100;

CREATE OR REPLACE VIEW public.max_users_list AS
SELECT
	u.id,
	CASE
		WHEN c.id IS NULL THEN NULL
		ELSE jsonb_build_object(
			'keys', jsonb_build_object('id', c.id),
			'descr', c.name
		)
	END AS customer,
	CASE
		WHEN sp.id IS NULL THEN NULL
		ELSE jsonb_build_object(
			'keys', jsonb_build_object('id', sp.id),
			'descr', sp.name
		)
	END AS customer_sale_place,
	u.max_user_id,
	u.username,
	u.avatar_url,
	u.raw_user,
	u.is_active,
	u.app_username
FROM public.max_users AS u
LEFT JOIN public.customers AS c ON c.id = u.customer_id
LEFT JOIN public.customer_sale_places AS sp ON sp.id = u.customer_sale_place_id;

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
FROM public.orders AS o
JOIN public.max_users AS u ON u.id = o.customer_user_id
WHERE o.customer_user_id IS NOT NULL;

COMMENT ON VIEW public.customer_users_list IS
	'Deduplicated MAX users that have placed orders for a customer.';

COMMIT;
