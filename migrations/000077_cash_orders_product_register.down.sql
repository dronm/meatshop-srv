BEGIN;

LOCK TABLE public.ra_products, public.rg_products, public.rg_products_current
	IN ACCESS EXCLUSIVE MODE;

-- Restoring the old schema must never silently discard cash-order actions.
DO $guard$
BEGIN
	IF EXISTS (SELECT 1 FROM public.ra_products WHERE customer_sale_place_id IS NULL)
		OR EXISTS (SELECT 1 FROM public.rg_products WHERE customer_sale_place_id IS NULL)
		OR EXISTS (SELECT 1 FROM public.rg_products_current WHERE customer_sale_place_id IS NULL)
	THEN
		RAISE EXCEPTION 'Cannot roll back cash-order product registers while actions or balances without a sale place exist'
			USING ERRCODE = '23502',
				HINT = 'Resolve the affected orders through the application before rolling back; no register data was changed.';
	END IF;
END;
$guard$;

ALTER TABLE public.ra_products
	ALTER COLUMN customer_sale_place_id SET NOT NULL;

ALTER TABLE public.rg_products
	DROP CONSTRAINT rg_products_dimensions_key,
	ALTER COLUMN customer_sale_place_id SET NOT NULL,
	ADD CONSTRAINT rg_products_pkey PRIMARY KEY (
		customer_id,
		customer_sale_place_id,
		product_id,
		measure_unit_id,
		period_start
	);

ALTER TABLE public.rg_products_current
	DROP CONSTRAINT rg_products_current_dimensions_key,
	ALTER COLUMN customer_sale_place_id SET NOT NULL,
	ADD CONSTRAINT rg_products_current_pkey PRIMARY KEY (
		customer_id,
		customer_sale_place_id,
		product_id,
		measure_unit_id
	);

CREATE OR REPLACE FUNCTION public.rg_products_apply_delta(
	in_effective_at timestamptz,
	in_customer_id integer,
	in_customer_sale_place_id integer,
	in_product_id integer,
	in_measure_unit_id integer,
	in_delta_quant_required numeric(19, 4),
	in_delta_quant numeric(19, 4)
)
RETURNS void
LANGUAGE plpgsql
AS $function$
DECLARE
	v_period_start date;
BEGIN
	IF in_delta_quant_required = 0 AND in_delta_quant = 0 THEN
		RETURN;
	END IF;

	v_period_start := public.register_month_start(in_effective_at);

	INSERT INTO public.rg_products (
		period_start,
		customer_id,
		customer_sale_place_id,
		product_id,
		measure_unit_id,
		quant_required,
		quant
	)
	VALUES (
		v_period_start,
		in_customer_id,
		in_customer_sale_place_id,
		in_product_id,
		in_measure_unit_id,
		in_delta_quant_required,
		in_delta_quant
	)
	ON CONFLICT (
		customer_id,
		customer_sale_place_id,
		product_id,
		measure_unit_id,
		period_start
	)
	DO UPDATE
	SET
		quant_required = rg_products.quant_required + EXCLUDED.quant_required,
		quant = rg_products.quant + EXCLUDED.quant;

	DELETE FROM public.rg_products
	WHERE period_start = v_period_start
		AND customer_id = in_customer_id
		AND customer_sale_place_id = in_customer_sale_place_id
		AND product_id = in_product_id
		AND measure_unit_id = in_measure_unit_id
		AND quant_required = 0
		AND quant = 0;

	INSERT INTO public.rg_products_current (
		customer_id,
		customer_sale_place_id,
		product_id,
		measure_unit_id,
		quant_required,
		quant
	)
	VALUES (
		in_customer_id,
		in_customer_sale_place_id,
		in_product_id,
		in_measure_unit_id,
		in_delta_quant_required,
		in_delta_quant
	)
	ON CONFLICT (
		customer_id,
		customer_sale_place_id,
		product_id,
		measure_unit_id
	)
	DO UPDATE
	SET
		quant_required = rg_products_current.quant_required + EXCLUDED.quant_required,
		quant = rg_products_current.quant + EXCLUDED.quant;

	DELETE FROM public.rg_products_current
	WHERE customer_id = in_customer_id
		AND customer_sale_place_id = in_customer_sale_place_id
		AND product_id = in_product_id
		AND measure_unit_id = in_measure_unit_id
		AND quant_required = 0
		AND quant = 0;
END;
$function$;

COMMIT;
