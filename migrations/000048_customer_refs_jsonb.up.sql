BEGIN;

DROP VIEW IF EXISTS public.customers_list;
DROP INDEX IF EXISTS public.customers_ref_1c_idx;

ALTER TABLE public.customers
	ALTER COLUMN ref_1c TYPE jsonb
	USING CASE
		WHEN NULLIF(btrim(ref_1c), '') IS NULL THEN NULL
		ELSE jsonb_build_object(
			'id', btrim(ref_1c),
			'descr', name
		)
	END;

ALTER TABLE public.customer_sale_places
	ALTER COLUMN ref_1c TYPE jsonb
	USING CASE
		WHEN NULLIF(btrim(ref_1c), '') IS NULL THEN NULL
		ELSE jsonb_build_object(
			'id', btrim(ref_1c),
			'descr', name
		)
	END;

CREATE UNIQUE INDEX customers_ref_1c_idx
	ON public.customers USING btree ((ref_1c->>'id'));

WITH aliases(table_name, column_name, column_alias) AS (
	VALUES
		('customer_sale_places', 'kpp', 'КПП'),
		('customer_sale_places', 'ref_1c', 'Ссылка 1С')
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

CREATE VIEW public.customers_list AS
SELECT
	m.id,
	m.name,
	m.inn,
	m.kpp,
	NULLIF(btrim(m.ref_1c->>'id'), '') IS NOT NULL AS ref_1c_exists,
	m.is_active,
	m.allow_holiday_orders
FROM public.customers AS m;

COMMIT;
