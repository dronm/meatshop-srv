BEGIN;

DROP VIEW public.max_out_messages_list;

-- A user-only queue cannot represent chat destinations after rollback.
DELETE FROM public.max_out_messages WHERE max_chat_id IS NOT NULL;
UPDATE public.max_out_messages SET status = 'failed'
WHERE status = 'superseded';

ALTER TABLE public.max_out_messages
	DROP CONSTRAINT max_out_messages_destination_check,
	DROP CONSTRAINT max_out_messages_chat_id_check,
	DROP CONSTRAINT max_out_messages_sequence_check,
	DROP CONSTRAINT max_out_messages_status_check,
	ADD CONSTRAINT max_out_messages_status_check
		CHECK (status IN ('pending', 'sending', 'sent', 'failed')),
	ALTER COLUMN max_user_id SET NOT NULL;

DROP INDEX public.max_out_messages_delivery_key_idx;
DROP INDEX public.max_out_messages_sequence_idx;
DROP INDEX public.max_out_messages_max_chat_id_idx;
DROP INDEX public.max_out_messages_production_document_idx;

ALTER TABLE public.max_out_messages
	DROP COLUMN max_chat_id,
	DROP COLUMN remote_message_id,
	DROP COLUMN delivery_key,
	DROP COLUMN sequence_key,
	DROP COLUMN sequence_num;

CREATE VIEW public.max_out_messages_list AS
SELECT
	m.id, m.max_user_id, public.max_users_ref(mu) AS max_user,
	m.status, m.attempt_count, m.next_attempt_at, m.locked_at,
	m.sent_at, m.created_at, m.error_str
FROM public.max_out_messages AS m
LEFT JOIN public.max_users AS mu ON mu.max_user_id = m.max_user_id;

DELETE FROM public.webapp_constants WHERE code = 'production_max_chat_id';

COMMIT;
