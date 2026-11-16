BEGIN;

ALTER TABLE public.max_out_messages
	ALTER COLUMN max_user_id DROP NOT NULL,
	ADD COLUMN max_chat_id bigint,
	ADD COLUMN remote_message_id text,
	ADD COLUMN delivery_key text,
	ADD COLUMN sequence_key text,
	ADD COLUMN sequence_num integer,
	DROP CONSTRAINT max_out_messages_status_check,
	ADD CONSTRAINT max_out_messages_status_check
		CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'superseded')),
	ADD CONSTRAINT max_out_messages_destination_check
		CHECK ((max_user_id IS NULL) <> (max_chat_id IS NULL)),
	ADD CONSTRAINT max_out_messages_chat_id_check
		CHECK (max_chat_id IS NULL OR max_chat_id <> 0),
	ADD CONSTRAINT max_out_messages_sequence_check
		CHECK ((sequence_key IS NULL AND sequence_num IS NULL)
			OR (sequence_key IS NOT NULL AND sequence_num > 0));

CREATE UNIQUE INDEX max_out_messages_delivery_key_idx
	ON public.max_out_messages (delivery_key)
	WHERE delivery_key IS NOT NULL;
CREATE INDEX max_out_messages_sequence_idx
	ON public.max_out_messages (sequence_key, id)
	WHERE sequence_key IS NOT NULL AND status NOT IN ('sent', 'superseded');
CREATE INDEX max_out_messages_max_chat_id_idx
	ON public.max_out_messages (max_chat_id, created_at DESC)
	WHERE max_chat_id IS NOT NULL;
CREATE INDEX max_out_messages_production_document_idx
	ON public.max_out_messages ((metadata->>'production_transfer_id'), (metadata->>'version'))
	WHERE metadata->>'source' = 'production_transfer';

COMMENT ON COLUMN public.max_out_messages.max_chat_id IS 'MAX group or channel destination ID, mutually exclusive with max_user_id.';
COMMENT ON COLUMN public.max_out_messages.remote_message_id IS 'Message ID returned by MAX after delivery.';
COMMENT ON COLUMN public.max_out_messages.delivery_key IS 'Stable application delivery key; prevents duplicate document-version message queueing.';
COMMENT ON COLUMN public.max_out_messages.sequence_key IS 'Messages in a sequence are delivered in insertion order, including retries.';
COMMENT ON COLUMN public.max_out_messages.sequence_num IS 'One-based part number within a document version.';

CREATE OR REPLACE VIEW public.max_out_messages_list AS
SELECT
	m.id,
	m.max_user_id,
	public.max_users_ref(mu) AS max_user,
	m.status,
	m.attempt_count,
	m.next_attempt_at,
	m.locked_at,
	m.sent_at,
	m.created_at,
	m.error_str,
	m.max_chat_id::text AS max_chat_id,
	m.remote_message_id
FROM public.max_out_messages AS m
LEFT JOIN public.max_users AS mu ON mu.max_user_id = m.max_user_id;

INSERT INTO public.webapp_constants (code, name, description, value_type, value)
VALUES (
	'production_max_chat_id',
	'Чат MAX для производства',
	'Идентификатор общего чата или канала MAX для заявок в производство. Получите chat_id из события bot_added и укажите его полностью, включая знак. Бот должен иметь права отправки сообщений в этом чате.',
	'string',
	'null'::jsonb
)
ON CONFLICT (code) DO NOTHING;

COMMIT;
