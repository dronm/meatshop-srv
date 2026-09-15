BEGIN;

ALTER TABLE public.customers
	ADD COLUMN allow_holiday_orders boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN public.customers.allow_holiday_orders IS
	'Allows orders whose for_date is a holiday.';

CREATE TABLE public.order_calendar_days (
	calendar_date date NOT NULL PRIMARY KEY,
	is_holiday boolean NOT NULL DEFAULT true,
	name text,
	CONSTRAINT order_calendar_days_name_chk
		CHECK (name IS NULL OR btrim(name) <> '')
);

COMMENT ON TABLE public.order_calendar_days IS
	'Order calendar overrides. Sundays are holidays by default when no row exists.';
COMMENT ON COLUMN public.order_calendar_days.calendar_date IS
	'Business calendar date.';
COMMENT ON COLUMN public.order_calendar_days.is_holiday IS
	'Whether the date is a holiday; false overrides a default Sunday holiday.';
COMMENT ON COLUMN public.order_calendar_days.name IS
	'Optional holiday or working-day override description.';

INSERT INTO public.audit_column_aliases (
	table_name,
	column_name,
	column_alias,
	is_active
)
VALUES (
	'customers',
	'allow_holiday_orders',
	'Заказы в праздничные дни',
	true
)
ON CONFLICT (table_name, column_name) DO UPDATE
SET
	column_alias = EXCLUDED.column_alias,
	is_active = true;

CREATE OR REPLACE VIEW public.customers_list AS
SELECT
	m.id,
	m.name,
	m.inn,
	m.kpp,
	m.ref_1c IS NOT NULL AS ref_1c_exists,
	m.is_active,
	m.allow_holiday_orders
FROM public.customers AS m;

COMMIT;
