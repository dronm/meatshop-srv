BEGIN;

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
	m.error_str
FROM public.max_out_messages AS m
LEFT JOIN public.max_users AS mu
	ON mu.max_user_id = m.max_user_id;

COMMENT ON VIEW public.max_out_messages_list IS
	'Lightweight outgoing MAX message collection with a MAX user reference.';

CREATE OR REPLACE VIEW public.max_in_messages_list AS
SELECT
	m.id,
	m.update_type,
	m.max_user_id,
	public.max_users_ref(mu) AS max_user,
	m.max_chat_id,
	m.created_at
FROM public.max_in_messages AS m
LEFT JOIN public.max_users AS mu
	ON mu.max_user_id = m.max_user_id;

COMMENT ON VIEW public.max_in_messages_list IS
	'Lightweight incoming MAX message collection with an optional MAX user reference.';

COMMIT;
