BEGIN;

DROP VIEW IF EXISTS public.product_register_recorders;

DROP FUNCTION IF EXISTS public.rg_products_balance(timestamptz, integer[], integer[], integer[], integer[]);
DROP FUNCTION IF EXISTS public.rg_products_balance(integer[], integer[], integer[], integer[]);
DROP FUNCTION IF EXISTS public.rg_products_rebuild();
DROP FUNCTION IF EXISTS public.ra_products_remove_acts(text, bigint);
DROP FUNCTION IF EXISTS public.ra_products_add_act(timestamptz, text, bigint, integer, integer, integer, integer, numeric, numeric);

DROP TRIGGER IF EXISTS ra_products_reject_update_trigger ON public.ra_products;
DROP FUNCTION IF EXISTS public.ra_products_reject_update();

DROP TRIGGER IF EXISTS ra_products_process_trigger ON public.ra_products;
DROP FUNCTION IF EXISTS public.ra_products_process();
DROP FUNCTION IF EXISTS public.rg_products_apply_delta(timestamptz, integer, integer, integer, integer, numeric, numeric);

DROP TABLE IF EXISTS public.rg_products_current;
DROP TABLE IF EXISTS public.rg_products;
DROP TABLE IF EXISTS public.ra_products;

COMMIT;
