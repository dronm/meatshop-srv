BEGIN;

CREATE INDEX audit_log_object_history_idx
	ON public.audit_log (table_name, record_id, changed_at DESC, id DESC);

INSERT INTO public.permissions (code, description)
VALUES ('objectHistory.list', 'Просмотр истории изменений объектов')
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

INSERT INTO public.role_permissions (role_id, permission_code)
VALUES ('admin'::public.role_types, 'objectHistory.list')
ON CONFLICT DO NOTHING;

COMMIT;

