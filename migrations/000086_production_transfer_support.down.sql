BEGIN;
DROP TRIGGER IF EXISTS audit_log_production_transfer_orders ON public.production_transfer_orders;
DROP TRIGGER IF EXISTS audit_log_production_transfers ON public.production_transfers;
DELETE FROM public.audit_column_aliases WHERE table_name IN ('production_transfers','production_transfer_orders');
DROP VIEW IF EXISTS public.production_transfers_list;
DROP VIEW IF EXISTS public.production_transfer_orders_list;
DROP VIEW IF EXISTS public.production_order_options;
DROP FUNCTION IF EXISTS public.production_format_quant(numeric);
COMMIT;
