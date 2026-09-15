BEGIN;

DROP VIEW public.customers_list;

CREATE VIEW public.customers_list AS
SELECT
	m.id,
	m.name,
	m.inn,
	m.kpp,
	m.ref_1c IS NOT NULL AS ref_1c_exists,
	m.is_active
FROM public.customers AS m;

DELETE FROM public.audit_column_aliases
WHERE table_name = 'customers'
	AND column_name = 'allow_holiday_orders'
	AND column_alias = 'Заказы в праздничные дни';

DROP TABLE public.order_calendar_days;

ALTER TABLE public.customers
	DROP COLUMN allow_holiday_orders;

COMMIT;
