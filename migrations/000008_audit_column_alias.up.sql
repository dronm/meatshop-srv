BEGIN;

WITH aliases(table_name, column_name, column_alias) AS (
	VALUES
		-- Users.
		('users', 'id', 'Идентификатор'),
		('users', 'name', 'Имя пользователя'),
		('users', 'role_id', 'Роль'),
		('users', 'pwd', 'Пароль'),
		('users', 'create_dt', 'Дата создания'),
		('users', 'banned', 'Доступ запрещён')

)
INSERT INTO public.audit_column_aliases (
	table_name,
	column_name,
	column_alias,
	is_active
)
SELECT
	aliases.table_name,
	aliases.column_name,
	aliases.column_alias,
	true
FROM aliases
ON CONFLICT (table_name, column_name) DO UPDATE
SET
	column_alias = EXCLUDED.column_alias,
	is_active = true;

COMMIT;

