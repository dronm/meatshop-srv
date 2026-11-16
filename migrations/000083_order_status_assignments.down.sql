BEGIN;

DROP TRIGGER IF EXISTS orders_track_status_assignment ON public.orders;
DROP FUNCTION IF EXISTS public.orders_track_status_assignment();
DROP FUNCTION IF EXISTS public.order_status_assignments_recalculate(integer);
DROP VIEW IF EXISTS public.order_status_assignments_list;
DROP TRIGGER IF EXISTS order_status_assignments_notify ON public.order_status_assignments;
DROP FUNCTION IF EXISTS public.order_status_assignments_notify();
DROP TRIGGER IF EXISTS audit_log_order_status_assignments ON public.order_status_assignments;
DROP INDEX IF EXISTS public.order_status_assignments_active_recorder_idx;
DROP INDEX IF EXISTS public.order_status_assignments_effective_idx;
DROP INDEX IF EXISTS public.order_status_assignments_order_history_idx;
DELETE FROM public.audit_column_aliases WHERE table_name = 'order_status_assignments';
DELETE FROM public.role_permissions
WHERE permission_code IN ('orderStatusAssignment.list', 'orderStatusAssignment.detail');
DELETE FROM public.permissions
WHERE code IN ('orderStatusAssignment.list', 'orderStatusAssignment.detail');

COMMIT;
