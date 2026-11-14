BEGIN;

DELETE FROM public.main_menus WHERE route_id IN
	(SELECT id FROM public.application_routes WHERE name = 'customerMessages');
DELETE FROM public.application_routes WHERE name = 'customerMessages';
DELETE FROM public.role_permissions WHERE permission_code IN
	('customerMessage.list', 'customerMessage.detail', 'customerMessage.markRead');
DELETE FROM public.permissions WHERE code IN
	('customerMessage.list', 'customerMessage.detail', 'customerMessage.markRead');

DROP TRIGGER max_users_attach_customer_messages ON public.max_users;
DROP TRIGGER max_users_lock_customer_messages ON public.max_users;
DROP TRIGGER max_in_messages_store_customer_message ON public.max_in_messages;
DROP FUNCTION public.max_users_attach_customer_messages();
DROP FUNCTION public.max_users_lock_customer_messages();
DROP FUNCTION public.max_in_messages_store_customer_message();
DROP VIEW public.customer_messages_list;
DROP TABLE public.customer_message_reads;
DROP TABLE public.customer_messages;
DROP FUNCTION public.customer_messages_notify_insert();
DROP FUNCTION public.store_max_customer_message(bigint, text, bigint, bigint, jsonb, timestamptz);

COMMIT;
