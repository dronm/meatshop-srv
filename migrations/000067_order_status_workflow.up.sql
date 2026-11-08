BEGIN;

-- Preserve status IDs/codes and existing order references, including history.
UPDATE public.order_statuses AS status
SET name = workflow.name,
	sort_order = workflow.sort_order
FROM (VALUES
	('new', 'Новый', 10),
	('processing', 'В обработке', 20),
	('accepted', 'Готовим', 30),
	('approved', 'На доставке', 40),
	('completed', 'Выполнен', 60),
	('cancelled', 'Отменен', 60)
) AS workflow(code, name, sort_order)
WHERE status.code = workflow.code;

COMMIT;
