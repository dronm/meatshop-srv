BEGIN;

CREATE INDEX order_status_assignments_order_history_idx
	ON public.order_status_assignments(order_id, id DESC);
CREATE INDEX order_status_assignments_effective_idx
	ON public.order_status_assignments(order_id, id DESC) WHERE is_active;
CREATE UNIQUE INDEX order_status_assignments_active_recorder_idx
	ON public.order_status_assignments(order_id, recorder)
	WHERE is_active AND recorder IS NOT NULL;

COMMENT ON TABLE public.order_status_assignments IS
	'Order status assignments. Effective status is the latest active assignment by id.';
COMMENT ON COLUMN public.order_status_assignments.recorder IS
	'Null for a manual assignment; otherwise {document_type, document_id}.';

-- Existing state is the baseline, including an explicitly empty status.
-- Block concurrent writes until tracking is installed so no status transition
-- can fall between the baseline snapshot and CREATE TRIGGER.
LOCK TABLE public.orders IN SHARE ROW EXCLUSIVE MODE;
INSERT INTO public.order_status_assignments(order_id, status_id)
SELECT id, status_id FROM public.orders ORDER BY id;

CREATE FUNCTION public.orders_track_status_assignment()
RETURNS trigger LANGUAGE plpgsql AS $BODY$
BEGIN
	-- Recalculation must not turn a document-owned change into a manual change.
	IF current_setting('app.order_status_assignment_sync', true) = 'on' THEN
		RETURN NEW;
	END IF;
	IF TG_OP = 'UPDATE' AND NEW.status_id IS NOT DISTINCT FROM OLD.status_id THEN
		RETURN NEW;
	END IF;
	INSERT INTO public.order_status_assignments(order_id, status_id)
	VALUES (NEW.id, NEW.status_id);
	RETURN NEW;
END;
$BODY$;

CREATE TRIGGER orders_track_status_assignment
AFTER INSERT OR UPDATE OF status_id ON public.orders
FOR EACH ROW EXECUTE FUNCTION public.orders_track_status_assignment();

-- Caller holds the normal order/product-recorder locks. The row lock here also
-- protects direct callers; assignment mutations and recalculation share a tx.
CREATE FUNCTION public.order_status_assignments_recalculate(p_order_id integer)
RETURNS TABLE(status_changed boolean, order_version bigint, customer_user_id integer)
LANGUAGE plpgsql AS $BODY$
DECLARE
	old_status integer;
	effective_status integer;
	previous_sync_setting text;
BEGIN
	SELECT o.status_id, o.version, o.customer_user_id
	INTO old_status, order_version, customer_user_id
	FROM public.orders AS o WHERE o.id = p_order_id FOR UPDATE;
	IF NOT FOUND THEN
		RAISE EXCEPTION 'Заказ не найден: %', p_order_id USING ERRCODE = '23503';
	END IF;
	SELECT a.status_id INTO effective_status
	FROM public.order_status_assignments AS a
	WHERE a.order_id = p_order_id AND a.is_active
	ORDER BY a.id DESC LIMIT 1;
	status_changed := old_status IS DISTINCT FROM effective_status;
	IF status_changed THEN
		previous_sync_setting := current_setting('app.order_status_assignment_sync', true);
		PERFORM set_config('app.order_status_assignment_sync', 'on', true);
		UPDATE public.orders AS o
		SET status_id = effective_status, version = o.version + 1
		WHERE o.id = p_order_id RETURNING o.version INTO order_version;
		PERFORM set_config('app.order_status_assignment_sync', COALESCE(previous_sync_setting, ''), true);
	END IF;
	RETURN NEXT;
END;
$BODY$;

CREATE VIEW public.order_status_assignments_list AS
SELECT
	a.id, a.order_id, a.status_id,
	public.order_statuses_ref(s) AS status,
	a.recorder, a.is_active, a.recorded_at,
	a.is_active AND NOT EXISTS (
		SELECT 1 FROM public.order_status_assignments AS newer
		WHERE newer.order_id = a.order_id AND newer.is_active AND newer.id > a.id
	) AS is_current
FROM public.order_status_assignments AS a
LEFT JOIN public.order_statuses AS s ON s.id = a.status_id;

CREATE FUNCTION public.order_status_assignments_notify()
RETURNS trigger LANGUAGE plpgsql AS $BODY$
BEGIN
	PERFORM pg_notify('OrderStatusAssignment.Update', json_build_object(
		'result', json_build_object('order_id', COALESCE(NEW.order_id, OLD.order_id))
	)::text);
	RETURN NULL;
END;
$BODY$;
CREATE TRIGGER order_status_assignments_notify
AFTER INSERT OR UPDATE OR DELETE ON public.order_status_assignments
FOR EACH ROW EXECUTE FUNCTION public.order_status_assignments_notify();

INSERT INTO public.permissions(code, description) VALUES
	('orderStatusAssignment.list', 'Просмотр истории статусов заказа'),
	('orderStatusAssignment.detail', 'Просмотр записи статуса заказа')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;
INSERT INTO public.role_permissions(role_id, permission_code)
VALUES
	('admin'::public.role_types, 'orderStatusAssignment.list'),
	('admin'::public.role_types, 'orderStatusAssignment.detail')
ON CONFLICT DO NOTHING;

INSERT INTO public.audit_column_aliases(table_name, column_name, column_alias, is_active)
VALUES
	('order_status_assignments', 'id', 'ID', true),
	('order_status_assignments', 'order_id', 'Заказ', true),
	('order_status_assignments', 'status_id', 'Статус', true),
	('order_status_assignments', 'recorder', 'Документ-основание', true),
	('order_status_assignments', 'is_active', 'Действует', true),
	('order_status_assignments', 'recorded_at', 'Дата записи', true)
ON CONFLICT (table_name, column_name) DO UPDATE
SET column_alias = EXCLUDED.column_alias, is_active = true;
CREATE TRIGGER audit_log_order_status_assignments
AFTER INSERT OR UPDATE OR DELETE ON public.order_status_assignments
FOR EACH ROW EXECUTE FUNCTION public.audit_log_process();

COMMIT;
