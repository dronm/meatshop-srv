begin;
	DELETE FROM orders;
	DELETE FROM ra_products;
	DELETE FROM rg_products;
	DELETE FROM products CASCADE;

	DROP INDEX measure_units_okei_code_idx;
	ALTER TABLE measure_units DROP COLUMN IF EXISTS okei_code;
commit;
