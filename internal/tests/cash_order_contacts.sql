-- Run with psql -X -v ON_ERROR_STOP=1 -f internal/tests/cash_order_contacts.sql
-- against a NEW, DISPOSABLE database. This builds a minimal pre-76 schema;
-- existing application tables intentionally cause the fixture to fail.
\set ON_ERROR_STOP on

CREATE TABLE public.customers (id integer PRIMARY KEY, name text, use_marking boolean);
CREATE TABLE public.customer_sale_places (id integer PRIMARY KEY, name text);
CREATE TABLE public.max_users (id integer PRIMARY KEY, max_user_id bigint, username text, app_username text);
CREATE TABLE public.order_statuses (id integer PRIMARY KEY, name text);
CREATE TABLE public.products (id integer PRIMARY KEY, name text, requires_weight_confirmation boolean);
CREATE TABLE public.orders (
	id integer PRIMARY KEY,
	for_date date NOT NULL,
	number_1c text,
	ref_1c jsonb,
	customer_id integer NOT NULL REFERENCES public.customers(id),
	customer_sale_place_id integer NOT NULL REFERENCES public.customer_sale_places(id),
	customer_user_id integer REFERENCES public.max_users(id),
	status_id integer REFERENCES public.order_statuses(id),
	comment_customer text,
	comment_admin text
);
CREATE TABLE public.order_items (
	id integer PRIMARY KEY,
	order_id integer NOT NULL REFERENCES public.orders(id) ON DELETE CASCADE,
	line_num integer,
	product_id integer REFERENCES public.products(id),
	quant numeric,
	price numeric,
	amount numeric,
	vat_amount numeric,
	weight_confirmed boolean,
	mercury boolean
);
CREATE TABLE public.notification_templates (code text PRIMARY KEY, event text, body_template text);

\ir ../../migrations/sql/customers_ref.sql
\ir ../../migrations/sql/customer_sale_places_ref.sql
\ir ../../migrations/sql/customer_users_ref.sql
\ir ../../migrations/sql/order_statuses_ref.sql
\ir ../../migrations/sql/products_ref.sql
\ir ../../migrations/000049_orders_shipment_ref_1c.up.sql
\ir ../../migrations/000073_order_shipment_number_1c.up.sql

INSERT INTO public.customers VALUES (1, 'Counterparty', false);
INSERT INTO public.customer_sale_places VALUES (1, 'Ordinary sale place');
INSERT INTO public.products VALUES (1, 'Product', false);
INSERT INTO public.orders (id, for_date, customer_id, customer_sale_place_id)
VALUES (1, '2026-11-14', 1, 1);
INSERT INTO public.order_items (id, order_id, line_num, product_id, quant)
VALUES (1, 1, 1, 1, 1);
INSERT INTO public.notification_templates (code, event, body_template)
VALUES
	('changed', 'order.changed', E'Before\nТочка продажи: {{.SalePlaceName}}\nAfter'),
	('submitted', 'order.submitted', E'Before\nТочка продажи: {{.SalePlaceName}}\nAfter'),
	('custom', 'order.changed', 'Наш адрес: {{.SalePlaceName}} / custom text'),
	('conditional', 'order.submitted', E'Before{{if .SalePlaceName}}\nТочка продажи: {{.SalePlaceName}}{{end}}\nAfter'),
	('other_event', 'other', E'Before\nТочка продажи: {{.SalePlaceName}}\nAfter');

\ir ../../migrations/000076_cash_order_contacts.up.sql

DO $test$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM public.orders
		WHERE id = 1 AND cash_payed = false AND customer_name IS NULL AND customer_phone IS NULL
	) THEN
		RAISE EXCEPTION 'Existing ordinary order did not keep false/default empty contacts';
	END IF;
	INSERT INTO public.orders (id, for_date, customer_id, cash_payed, customer_name, customer_phone)
	VALUES (2, '2026-11-14', 1, true, 'Actual buyer', '+79991234567');
	INSERT INTO public.order_items (id, order_id, line_num, product_id, quant)
	VALUES (2, 2, 1, 1, 1);
	INSERT INTO public.orders (id, for_date, customer_id, cash_payed)
	VALUES (3, '2026-11-14', 1, true);
	IF NOT EXISTS (
		SELECT 1 FROM public.orders
		WHERE id = 3 AND customer_name IS NULL AND customer_phone IS NULL AND customer_sale_place_id IS NULL
	) THEN
		RAISE EXCEPTION 'Cash order did not allow optional empty contacts';
	END IF;
	INSERT INTO public.orders (id, for_date, customer_id, customer_sale_place_id)
	VALUES (4, '2026-11-14', 1, 1);
	IF NOT EXISTS (SELECT 1 FROM public.orders WHERE id = 4 AND cash_payed = false) THEN
		RAISE EXCEPTION 'Omitted cash flag did not default to false';
	END IF;
	BEGIN
		INSERT INTO public.orders (id, for_date, customer_id)
		VALUES (5, '2026-11-14', 1);
		RAISE EXCEPTION 'Noncash order unexpectedly accepted absent sale place';
	EXCEPTION WHEN check_violation THEN
		NULL;
	END;
	BEGIN
		UPDATE public.orders SET cash_payed = false WHERE id = 2;
		RAISE EXCEPTION 'Switching cash off unexpectedly accepted absent sale place';
	EXCEPTION WHEN check_violation THEN
		NULL;
	END;
	IF NOT EXISTS (
		SELECT 1 FROM public.orders_list
		WHERE id = 2 AND cash_payed = true AND customer_name = 'Actual buyer'
			AND customer_phone = '+79991234567' AND customer_sale_place IS NULL
			AND customer->'keys'->>'id' = '1'
	) THEN
		RAISE EXCEPTION 'Order list omitted cash order or its nullable contact/place fields';
	END IF;
	IF NOT EXISTS (
		SELECT 1 FROM public.order_lines_list
		WHERE id = 2 AND item_id = 2 AND cash_payed = true
			AND customer_name = 'Actual buyer' AND customer_phone = '+79991234567'
			AND customer_sale_place IS NULL
	) THEN
		RAISE EXCEPTION 'Order line list omitted cash order or contact/place fields';
	END IF;
	IF (SELECT count(*) FROM public.notification_templates
		WHERE code IN ('changed', 'submitted', 'conditional')
			AND body_template = E'Before{{if .SalePlaceName}}\nТочка продажи: {{.SalePlaceName}}{{end}}\nAfter') <> 3
	THEN
		RAISE EXCEPTION 'Standard notification line was not wrapped once for both order events';
	END IF;
	IF (SELECT body_template FROM public.notification_templates WHERE code = 'custom')
		IS DISTINCT FROM 'Наш адрес: {{.SalePlaceName}} / custom text'
	THEN
		RAISE EXCEPTION 'Custom notification text was changed';
	END IF;
	IF (SELECT body_template FROM public.notification_templates WHERE code = 'other_event')
		IS DISTINCT FROM E'Before\nТочка продажи: {{.SalePlaceName}}\nAfter'
	THEN
		RAISE EXCEPTION 'Unrelated notification event was changed';
	END IF;
END;
$test$;

-- Resolve only fixture cash rows so the successful rollback path can be tested.
DELETE FROM public.orders WHERE id IN (2, 3);

\ir ../../migrations/000076_cash_order_contacts.down.sql

DO $test$
BEGIN
	IF EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'orders'
			AND column_name IN ('cash_payed', 'customer_name', 'customer_phone')
	) THEN
		RAISE EXCEPTION 'Rollback left cash contact columns behind';
	END IF;
	IF NOT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'orders'
			AND column_name = 'customer_sale_place_id' AND is_nullable = 'NO'
	) THEN
		RAISE EXCEPTION 'Rollback did not restore mandatory sale place';
	END IF;
END;
$test$;

-- Repeat after rollback to verify a previously conditional template is kept.
\ir ../../migrations/000076_cash_order_contacts.up.sql

DO $test$
BEGIN
	IF (SELECT count(*) FROM public.notification_templates
		WHERE code IN ('changed', 'submitted', 'conditional')
			AND body_template = E'Before{{if .SalePlaceName}}\nТочка продажи: {{.SalePlaceName}}{{end}}\nAfter') <> 3
	THEN
		RAISE EXCEPTION 'Reapplying migration duplicated notification conditions';
	END IF;
END;
$test$;
