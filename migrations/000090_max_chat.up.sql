BEGIN;

-- The queue identifier serves as the outgoing message identifier, but is not a
-- foreign key. Queue deletion (including a MAX user's cascade) must keep history.
CREATE TABLE public.max_chat_outgoing (
	id bigint PRIMARY KEY,
	customer_id integer REFERENCES public.customers(id) ON DELETE SET NULL,
	max_user_id bigint NOT NULL,
	kind text NOT NULL CHECK (kind IN ('manual', 'automatic')),
	author_user_id integer REFERENCES public.users(id) ON DELETE SET NULL,
	author_name text,
	request_id uuid,
	text text NOT NULL CHECK (text ~ '[^[:space:]]'),
	created_at timestamptz NOT NULL DEFAULT now(),
	status text NOT NULL DEFAULT 'pending'
		CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'superseded', 'unconfirmed')),
	sent_at timestamptz,
	remote_message_id text,
	error_str text,
	CHECK (
		(kind = 'manual' AND request_id IS NOT NULL AND author_name IS NOT NULL)
		OR (kind = 'automatic' AND request_id IS NULL AND author_user_id IS NULL AND author_name IS NULL)
	),
	UNIQUE (author_user_id, request_id)
);

CREATE INDEX max_chat_outgoing_history_idx ON public.max_chat_outgoing
	(customer_id, max_user_id, created_at DESC, id DESC);
CREATE INDEX customer_messages_chat_history_idx ON public.customer_messages
	(customer_id, max_user_id, sent_at DESC, id DESC);

-- The original order id is stored by the notification sender. For old queue
-- records lacking an audience marker, require both the order's MAX recipient
-- and its customer to agree with the existing account. The only legacy event
-- sent to the customer was order.changed.
CREATE VIEW public.max_chat_automatic_orders AS
SELECT q.id, o.customer_id, q.max_user_id, q.message->>'text' AS text,
	q.created_at, q.status, q.sent_at, q.remote_message_id, q.error_str
FROM public.max_out_messages AS q
JOIN public.orders AS o ON o.id = CASE
	WHEN q.metadata->>'entity_id' ~ '^[1-9][0-9]{0,9}$'
	THEN (q.metadata->>'entity_id')::bigint END
JOIN public.max_users AS u ON u.max_user_id = q.max_user_id
WHERE q.metadata->>'entity_type' = 'order'
	AND jsonb_typeof(q.message->'text') = 'string'
	AND q.message->>'text' ~ '[^[:space:]]'
	AND (
		(q.metadata->>'audience' = 'customer' AND q.metadata->>'event' IN ('order.changed', 'order.submitted'))
		OR (
			NOT (q.metadata ? 'audience') AND q.metadata->>'event' = 'order.changed'
			AND o.customer_user_id = u.id AND o.customer_id = u.customer_id
		)
	);

CREATE FUNCTION public.max_chat_capture_automatic() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	INSERT INTO public.max_chat_outgoing
		(id, customer_id, max_user_id, kind, text, created_at, status, sent_at, remote_message_id, error_str)
	SELECT id, customer_id, max_user_id, 'automatic', text, created_at, status, sent_at, remote_message_id, error_str
	FROM public.max_chat_automatic_orders WHERE id = NEW.id;
	RETURN NULL;
END;
$$;

CREATE TRIGGER max_chat_capture_automatic
AFTER INSERT ON public.max_out_messages
FOR EACH ROW WHEN (NEW.max_user_id IS NOT NULL AND NEW.metadata->>'entity_type' = 'order')
EXECUTE FUNCTION public.max_chat_capture_automatic();

CREATE FUNCTION public.max_chat_update_delivery() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF TG_OP = 'DELETE' THEN
		UPDATE public.max_chat_outgoing
		SET status = CASE WHEN OLD.attempt_count = 0 AND OLD.status = 'pending' THEN 'failed' ELSE 'unconfirmed' END,
			error_str = CASE WHEN OLD.attempt_count = 0 AND OLD.status = 'pending'
				THEN 'Сообщение удалено из очереди до отправки.'
				ELSE 'Сообщение удалено из очереди. Результат отправки неизвестен.' END
		WHERE id = OLD.id AND status IN ('pending', 'sending');
	ELSE
		UPDATE public.max_chat_outgoing
		SET status = NEW.status, sent_at = NEW.sent_at, remote_message_id = NEW.remote_message_id, error_str = NEW.error_str
		WHERE id = NEW.id;
	END IF;
	RETURN NULL;
END;
$$;

CREATE TRIGGER max_chat_update_delivery
AFTER UPDATE OF status, sent_at, remote_message_id, error_str ON public.max_out_messages
FOR EACH ROW WHEN ((OLD.status, OLD.sent_at, OLD.remote_message_id, OLD.error_str)
	IS DISTINCT FROM (NEW.status, NEW.sent_at, NEW.remote_message_id, NEW.error_str))
EXECUTE FUNCTION public.max_chat_update_delivery();

CREATE TRIGGER max_chat_remove_queued
AFTER DELETE ON public.max_out_messages
FOR EACH ROW WHEN (OLD.max_user_id IS NOT NULL AND OLD.status IN ('pending', 'sending'))
EXECUTE FUNCTION public.max_chat_update_delivery();

-- Trigger installation above holds the queue's DDL lock through commit. The
-- backfill and live capture therefore have no overlap or missed insert window.
INSERT INTO public.max_chat_outgoing
	(id, customer_id, max_user_id, kind, text, created_at, status, sent_at, remote_message_id, error_str)
SELECT id, customer_id, max_user_id, 'automatic', text, created_at, status, sent_at, remote_message_id, error_str
FROM public.max_chat_automatic_orders;

CREATE VIEW public.max_chat_history AS
SELECT customer_id, max_user_id, sent_at AS message_at, 0 AS direction_sort, id,
	'incoming'::text AS kind, text, sender_name AS author_name, NULL::text AS status,
	sent_at, max_message_id AS remote_message_id, NULL::text AS error_str
FROM public.customer_messages
UNION ALL
SELECT customer_id, max_user_id, created_at AS message_at, 1 AS direction_sort, id,
	kind, text, author_name, status, sent_at, remote_message_id, error_str
FROM public.max_chat_outgoing;

CREATE FUNCTION public.max_chat_notify_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	PERFORM pg_notify('CustomerMessage.ChatUpdate', jsonb_build_object(
		'id', NEW.id::text, 'customer_id', NEW.customer_id, 'max_user_id', NEW.max_user_id::text
	)::text);
	RETURN NULL;
END;
$$;

-- The existing incoming-message notifications and per-admin read receipts
-- still operate only on customer_messages.
CREATE TRIGGER max_chat_notify_update
AFTER INSERT OR UPDATE ON public.max_chat_outgoing
FOR EACH ROW EXECUTE FUNCTION public.max_chat_notify_update();

INSERT INTO public.permissions (code, description)
VALUES ('customerMessage.send', 'Отправка сообщений покупателям в MAX')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO public.role_permissions (role_id, permission_code)
VALUES ('admin'::public.role_types, 'customerMessage.send')
ON CONFLICT DO NOTHING;

INSERT INTO public.application_routes (name, path, descr, section, icon, menu_available, is_active)
VALUES ('maxChat', '/max-chat', 'Чат MAX', 'Покупатели', 'pi pi-comments', true, true)
ON CONFLICT (name) DO UPDATE SET path = EXCLUDED.path, descr = EXCLUDED.descr,
	section = EXCLUDED.section, icon = EXCLUDED.icon, menu_available = true, is_active = true;

INSERT INTO public.main_menus (role_id, parent_id, caption, route_id, icon, sort_order, is_active)
SELECT 'admin'::public.role_types, COALESCE(
	(SELECT m.parent_id FROM public.main_menus AS m JOIN public.application_routes AS r ON r.id = m.route_id
	 WHERE m.role_id = 'admin' AND m.user_id IS NULL AND r.name = 'customerMessages'
	 ORDER BY m.id LIMIT 1),
	(SELECT id FROM public.main_menus WHERE role_id = 'admin' AND user_id IS NULL
		AND parent_id IS NULL AND caption = 'Пользователи MAX' ORDER BY id LIMIT 1)
), 'Чат MAX', r.id, 'pi pi-comments', 30, true
FROM public.application_routes AS r
WHERE r.name = 'maxChat' AND NOT EXISTS (
	SELECT 1 FROM public.main_menus AS m WHERE m.role_id = 'admin' AND m.user_id IS NULL AND m.route_id = r.id
);

COMMIT;
