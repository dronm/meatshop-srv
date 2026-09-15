BEGIN;

DROP TRIGGER IF EXISTS audit_log_order_calendar_days
	ON public.order_calendar_days;

WITH aliases(table_name, column_name, column_alias) AS (
	VALUES
		('order_calendar_days', 'id', 'ID'),
		('order_calendar_days', 'calendar_date', 'Дата календаря'),
		('order_calendar_days', 'is_holiday', 'Выходной день'),
		('order_calendar_days', 'name', 'Описание')
)
DELETE FROM public.audit_column_aliases AS target
USING aliases
WHERE target.table_name = aliases.table_name
	AND target.column_name = aliases.column_name
	AND target.column_alias = aliases.column_alias;

DELETE FROM public.role_permissions
WHERE permission_code IN (
	'orderCalendarDay.create',
	'orderCalendarDay.list',
	'orderCalendarDay.detail',
	'orderCalendarDay.update',
	'orderCalendarDay.delete'
);

DELETE FROM public.permissions
WHERE code IN (
	'orderCalendarDay.create',
	'orderCalendarDay.list',
	'orderCalendarDay.detail',
	'orderCalendarDay.update',
	'orderCalendarDay.delete'
);

ALTER TABLE public.order_calendar_days
	DROP CONSTRAINT order_calendar_days_pkey,
	DROP CONSTRAINT order_calendar_days_calendar_date_key,
	ADD CONSTRAINT order_calendar_days_pkey PRIMARY KEY (calendar_date);

ALTER TABLE public.order_calendar_days
	DROP COLUMN id;

COMMIT;
