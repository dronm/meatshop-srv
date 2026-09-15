BEGIN;

	CREATE TABLE audit_log(
		id BIGSERIAL PRIMARY KEY,
		table_name TEXT NOT NULL,
		record_id TEXT NOT NULL, -- or JSONB for composite keys
		operation CHAR(1) NOT NULL,
		changes JSONB, -- Only changed columns: {"column": {"old": value, "new": value}}
		changed_at TIMESTAMPTZ DEFAULT NOW(),
		changed_by TEXT --user name
	);

	-- Create index for performance
	CREATE INDEX idx_audit_log_table_operation ON audit_log(table_name, record_id);
	CREATE INDEX idx_audit_log_changed_at ON audit_log(changed_at);

	-- Simple table for column aliases
	CREATE TABLE audit_column_aliases (
		id SERIAL PRIMARY KEY,
		table_name TEXT NOT NULL,
		column_name TEXT NOT NULL,
		column_alias TEXT NOT NULL,
		is_active BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		UNIQUE(table_name, column_name)
	);

	-- Create index for performance
	CREATE INDEX idx_audit_column_aliases_lookup 
	ON audit_column_aliases(table_name, column_name) 
	WHERE is_active = TRUE;

	-- Add some sample aliases
	INSERT INTO audit_column_aliases (table_name, column_name, column_alias) VALUES
	('suppliers', 'name', 'Наименование'),
	('suppliers', 'name_full', 'Полное Наименоание');

COMMIT;

