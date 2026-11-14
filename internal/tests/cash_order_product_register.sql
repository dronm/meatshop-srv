-- Run with psql -X -v ON_ERROR_STOP=1 -f internal/tests/cash_order_product_register.sql
-- against a NEW, DISPOSABLE database. Existing application tables intentionally
-- cause this fixture to fail instead of modifying production reference data.
\set ON_ERROR_STOP on

CREATE TABLE public.customers (id integer PRIMARY KEY);
CREATE TABLE public.customer_sale_places (id integer PRIMARY KEY);
CREATE TABLE public.products (id integer PRIMARY KEY);
CREATE TABLE public.measure_units (id integer PRIMARY KEY);
CREATE TABLE public.orders (id integer PRIMARY KEY, for_date date, number_1c text);

INSERT INTO public.customers VALUES (1);
INSERT INTO public.customer_sale_places VALUES (1);
INSERT INTO public.products VALUES (1);
INSERT INTO public.measure_units VALUES (1);

\ir ../../migrations/000018_register_common.up.sql
\ir ../../migrations/000019_create_products_register.up.sql
\ir ../../migrations/000077_cash_orders_product_register.up.sql

BEGIN;

DO $test$
DECLARE
	v_count bigint;
	v_required numeric;
	v_actual numeric;
BEGIN
	-- Several cash orders share one NULL dimension and accumulate exactly once.
	PERFORM public.ra_products_add_act('2026-10-10 00:00:00+00', 'CashTest', 1, 1, NULL, 1, 1, 2, 1);
	PERFORM public.ra_products_add_act('2026-10-11 00:00:00+00', 'CashTest', 2, 1, NULL, 1, 1, 3, 2);
	SELECT count(*), sum(quant_required), sum(quant)
	INTO v_count, v_required, v_actual
	FROM public.rg_products_current
	WHERE customer_sale_place_id IS NULL;
	IF v_count <> 1 OR v_required <> 5 OR v_actual <> 3 THEN
		RAISE EXCEPTION 'NULL current dimensions were duplicated or totals were incorrect';
	END IF;
	SELECT count(*), sum(quant_required), sum(quant)
	INTO v_count, v_required, v_actual
	FROM public.rg_products
	WHERE customer_sale_place_id IS NULL;
	IF v_count <> 1 OR v_required <> 5 OR v_actual <> 3 THEN
		RAISE EXCEPTION 'NULL monthly dimensions were duplicated or totals were incorrect';
	END IF;

	-- A real sale place remains a separate balance even for the same customer.
	PERFORM public.ra_products_add_act('2026-10-12 00:00:00+00', 'CashTest', 3, 1, 1, 1, 1, 7, 6);
	PERFORM public.ra_products_add_act('2026-11-01 00:00:00+00', 'CashTest', 4, 1, NULL, 1, 1, 11, 10);
	SELECT count(*), sum(quant_required), sum(quant)
	INTO v_count, v_required, v_actual
	FROM public.rg_products_balance()
	WHERE customer_sale_place_id IS NULL;
	IF v_count <> 1 OR v_required <> 16 OR v_actual <> 13 THEN
		RAISE EXCEPTION 'Current balance omitted or mixed cash dimensions';
	END IF;
	SELECT count(*), sum(quant_required), sum(quant)
	INTO v_count, v_required, v_actual
	FROM public.rg_products_balance('2026-11-02 00:00:00+00'::timestamptz)
	WHERE customer_sale_place_id IS NULL;
	IF v_count <> 1 OR v_required <> 16 OR v_actual <> 13 THEN
		RAISE EXCEPTION 'Historical balance omitted cash monthly/raw movements';
	END IF;
	SELECT count(*), sum(quant_required), sum(quant)
	INTO v_count, v_required, v_actual
	FROM public.rg_products_balance('2026-10-10 12:00:00+00'::timestamptz)
	WHERE customer_sale_place_id IS NULL;
	IF v_count <> 1 OR v_required <> 2 OR v_actual <> 1 THEN
		RAISE EXCEPTION 'Historical balance included later cash movements';
	END IF;
	IF NOT EXISTS (
		SELECT 1 FROM public.rg_products_current
		WHERE customer_sale_place_id = 1 AND quant_required = 7 AND quant = 6
	) THEN
		RAISE EXCEPTION 'Ordinary order balance was changed';
	END IF;

	-- Rebuild must preserve grouping of NULL dimensions and all totals.
	PERFORM public.rg_products_rebuild();
	IF (SELECT count(*) FROM public.rg_products WHERE customer_sale_place_id IS NULL) <> 2
		OR (SELECT count(*) FROM public.rg_products_current WHERE customer_sale_place_id IS NULL) <> 1
		OR NOT EXISTS (
			SELECT 1 FROM public.rg_products_current
			WHERE customer_sale_place_id IS NULL AND quant_required = 16 AND quant = 13
		)
	THEN
		RAISE EXCEPTION 'Register rebuild changed cash dimensions or quantities';
	END IF;

	-- Replacing an order removes its old contribution, with no zero rows left.
	PERFORM public.ra_products_remove_acts('CashTest', 1);
	PERFORM public.ra_products_add_act('2026-10-10 00:00:00+00', 'CashTest', 1, 1, NULL, 1, 1, 4, 3);
	IF NOT EXISTS (
		SELECT 1 FROM public.rg_products_current
		WHERE customer_sale_place_id IS NULL AND quant_required = 18 AND quant = 15
	) THEN
		RAISE EXCEPTION 'Replacing a cash order counted old quantities twice';
	END IF;
	PERFORM public.ra_products_remove_acts('CashTest', 1);
	PERFORM public.ra_products_remove_acts('CashTest', 2);
	PERFORM public.ra_products_remove_acts('CashTest', 4);
	IF EXISTS (SELECT 1 FROM public.rg_products WHERE customer_sale_place_id IS NULL)
		OR EXISTS (SELECT 1 FROM public.rg_products_current WHERE customer_sale_place_id IS NULL)
	THEN
		RAISE EXCEPTION 'Removing cash orders left zero aggregate rows';
	END IF;
	PERFORM public.ra_products_remove_acts('CashTest', 3);
	IF EXISTS (SELECT 1 FROM public.ra_products)
		OR EXISTS (SELECT 1 FROM public.rg_products)
		OR EXISTS (SELECT 1 FROM public.rg_products_current)
	THEN
		RAISE EXCEPTION 'Removing all test orders did not empty the register';
	END IF;
END;
$test$;

ROLLBACK;
