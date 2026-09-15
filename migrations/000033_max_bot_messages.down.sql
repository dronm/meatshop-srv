BEGIN;

DELETE FROM public.role_permissions
WHERE role_id = 'admin'::public.role_types
	AND permission_code = 'maxNotification.send';

DELETE FROM public.permissions
WHERE code = 'maxNotification.send';

DROP TRIGGER IF EXISTS max_out_messages_notify_insert ON public.max_out_messages;
DROP FUNCTION IF EXISTS public.max_out_messages_notify_insert();

DROP TABLE IF EXISTS public.max_out_messages;
DROP TABLE IF EXISTS public.max_in_messages;

COMMIT;
