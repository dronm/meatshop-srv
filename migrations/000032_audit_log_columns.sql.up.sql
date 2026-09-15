BEGIN;
	DELETE FROM audit_column_aliases where table_name='suppliers';

	INSERT INTO audit_column_aliases (table_name, column_name, column_alias) VALUES
	('customers', 'id', 'Идентификатор'),
	('customers', 'name', 'Наименование'),
	('customers', 'inn', 'ИНН'),
	('customers', 'kpp', 'КПП'),
	('customers', 'ref_1c', 'Ссылка 1с'),
	('customers', 'is_active', 'Используется')
	ON CONFLICT (table_name, column_name) DO UPDATE SET column_alias = excluded.column_alias;

	INSERT INTO audit_column_aliases (table_name, column_name, column_alias) VALUES
	('customer_sale_places', 'id', 'Идентификатор'),
	('customer_sale_places', 'name', 'Наименование'),
	('customer_sale_places', 'address', 'Адрес')
	ON CONFLICT (table_name, column_name) DO UPDATE SET column_alias = excluded.column_alias;

	INSERT INTO audit_column_aliases (table_name, column_name, column_alias) VALUES
	('max_users', 'id', 'Идентификатор'),
	('max_users', 'customer_id', 'Покупатель'),
	('max_users', 'max_user_id', 'ID MAX'),
	('max_users', 'avatar_url', 'Аватар'),
	('max_users', 'is_active', 'Используется')
	ON CONFLICT (table_name, column_name) DO UPDATE SET column_alias = excluded.column_alias;

	INSERT INTO audit_column_aliases (table_name, column_name, column_alias) VALUES
	('orders', 'id', 'Идентификатор'),
	('orders', 'for_date', 'На дату'),
	('orders', 'ref_1c', 'Ссыока 1с'),
	('orders', 'customer_id', 'Покупатель'),
	('orders', 'customer_sale_place_id', 'Точка'),
	('orders', 'customer_user_id', 'Пользователь покупателя'),
	('orders', 'status_id', 'Статус'),
	('orders', 'comment_customer', 'Комментарий заказчика'),
	('orders', 'comment_admin', 'Комментарий администратора')
	ON CONFLICT (table_name, column_name) DO UPDATE SET column_alias = excluded.column_alias;

	INSERT INTO audit_column_aliases (table_name, column_name, column_alias) VALUES
	('order_items', 'id', 'Идентификатор'),
	('order_items', 'line_num', 'Строка'),
	('order_items', 'product_id', 'Номенклатура'),
	('order_items', 'measure_unit_id', 'Единица'),
	('order_items', 'quant_required', 'Кол-во запрошено'),
	('order_items', 'quant', 'Кол-во')
	ON CONFLICT (table_name, column_name) DO UPDATE SET column_alias = excluded.column_alias;

	INSERT INTO audit_column_aliases (table_name, column_name, column_alias) VALUES
	('order_statuses', 'id', 'Идентификатор'),
	('order_statuses', 'code', 'Код'),
	('order_statuses', 'sort_order', 'Порядок'),
	('order_statuses', 'is_final', 'Финальный')
	ON CONFLICT (table_name, column_name) DO UPDATE SET column_alias = excluded.column_alias;
COMMIT;
