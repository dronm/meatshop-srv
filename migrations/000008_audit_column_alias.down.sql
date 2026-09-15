BEGIN;

WITH aliases(table_name, column_name, column_alias) AS (
	VALUES
		('users', 'id', 'Идентификатор'),
		('users', 'name', 'Имя пользователя'),
		('users', 'role_id', 'Роль'),
		('users', 'pwd', 'Пароль'),
		('users', 'create_dt', 'Дата создания'),
		('users', 'banned', 'Доступ запрещён')
)
DELETE FROM public.audit_column_aliases AS target
USING aliases
WHERE target.table_name = aliases.table_name
	AND target.column_name = aliases.column_name
	AND target.column_alias = aliases.column_alias;

COMMIT;

