BEGIN;

UPDATE public.order_statuses AS status
SET name = workflow.name,
	sort_order = workflow.sort_order
FROM (VALUES
	('new', 'Новый', 10),
	('accepted', 'Принят', 20),
	('processing', 'В обработке', 40),
	('approved', 'Согласован', 50),
	('completed', 'Выполнен', 60),
	('cancelled', 'Отменен', 60)
) AS workflow(code, name, sort_order)
WHERE status.code = workflow.code;

COMMIT;
