BEGIN;

DELETE FROM public.role_permissions
WHERE permission_code = 'objectHistory.list';

DELETE FROM public.permissions
WHERE code = 'objectHistory.list';

DROP INDEX IF EXISTS public.audit_log_object_history_idx;

COMMIT;

