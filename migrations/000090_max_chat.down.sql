BEGIN;

DELETE FROM public.main_menus
WHERE route_id IN (SELECT id FROM public.application_routes WHERE name = 'maxChat');
DELETE FROM public.application_routes WHERE name = 'maxChat';
DELETE FROM public.role_permissions WHERE permission_code = 'customerMessage.send';
DELETE FROM public.permissions WHERE code = 'customerMessage.send';

DROP TRIGGER max_chat_notify_update ON public.max_chat_outgoing;
DROP FUNCTION public.max_chat_notify_update();
DROP VIEW public.max_chat_history;
DROP TRIGGER max_chat_remove_queued ON public.max_out_messages;
DROP TRIGGER max_chat_update_delivery ON public.max_out_messages;
DROP FUNCTION public.max_chat_update_delivery();
DROP TRIGGER max_chat_capture_automatic ON public.max_out_messages;
DROP FUNCTION public.max_chat_capture_automatic();
DROP VIEW public.max_chat_automatic_orders;
DROP INDEX public.customer_messages_chat_history_idx;
DROP TABLE public.max_chat_outgoing;

COMMIT;
