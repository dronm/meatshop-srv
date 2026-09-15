-- Import product groups and products from Продукция.xlsx
-- Mapping:
--   Артикул производителя -> products.sort_order
--   Сортировочная группа  -> parent product group
--   Вид продукции         -> products.name
--
-- Groups are inserted first as root products with is_group = true.
-- Group sort_order is the smallest manufacturer article in that group.
-- Products are inserted under the corresponding group with is_group = false.
-- The migration is idempotent by normalized group/product name within the expected hierarchy.
BEGIN;

-- Measure units.
ALTER TABLE measure_units
	ADD COLUMN IF NOT EXISTS okei_code character varying(4);

-- Fill codes for units that may already exist before adding the constraint.
UPDATE measure_units
SET okei_code = '796'
WHERE lower(btrim(name)) = lower('шт')
	AND okei_code IS DISTINCT FROM '796';

UPDATE measure_units
SET okei_code = '166'
WHERE lower(btrim(name)) = lower('кг')
	AND okei_code IS DISTINCT FROM '166';

UPDATE measure_units
SET okei_code = '8751'
WHERE lower(btrim(name)) = lower('кор')
	AND okei_code IS DISTINCT FROM '8751';

CREATE UNIQUE INDEX IF NOT EXISTS measure_units_okei_code_idx
	ON measure_units (okei_code);

INSERT INTO measure_units (
	name,
	name_full,
	okei_code
)
VALUES
	('кг', 'Килограмм', '166'),
	('кор', 'Коробка', '8751')
ON CONFLICT (okei_code) DO NOTHING;

ALTER TABLE measure_units
	ALTER COLUMN okei_code SET NOT NULL;

-- Keep the source list in a temporary table so it can be reused for groups,
-- updates of existing products, and insertion of missing products.
CREATE TEMPORARY TABLE migration_product_seed (
	sort_order integer NOT NULL,
	product_name text NOT NULL,
	group_name text NOT NULL
) ON COMMIT DROP;

INSERT INTO migration_product_seed (
	sort_order,
	product_name,
	group_name
)
VALUES
	(1, 'Тушка цыпленка-бройлера 1-го сорта охлажденная', 'Охлажденная мясная продукция'),
	(2, 'Тушка цыпленка-бройлера 1-го сорта замороженная', 'Замороженная мясная продукция'),
	(3, 'Грудка цыпленка-бройлера на кости охлажденная', 'Охлажденная мясная продукция'),
	(4, 'Грудка цыпленка-бройлера на кости замороженная', 'Замороженная мясная продукция'),
	(5, 'Окорочок цыпленка-бройлера на кости охлажденный', 'Охлажденная мясная продукция'),
	(6, 'Окорочок цыпленка-бройлера на кости замороженный', 'Замороженная мясная продукция'),
	(7, 'Голень цыпленка-бройлера охлажденная', 'Охлажденная мясная продукция'),
	(8, 'Голень цыпленка-бройлера замороженная', 'Замороженная мясная продукция'),
	(9, 'Бедро цыпленка-бройлера охлажденное', 'Охлажденная мясная продукция'),
	(10, 'Бедро цыпленка-бройлера замороженное', 'Замороженная мясная продукция'),
	(11, 'Крылья цыпленка-бройлера охлажденные 3ф', 'Охлажденная мясная продукция'),
	(12, 'Крылья цыпленка-бройлера замороженные 3ф', 'Замороженная мясная продукция'),
	(13, 'Филе грудки цыпленка-бройлера с кожей охлажденное', 'Охлажденная мясная продукция'),
	(14, 'Филе грудки цыпленка-бройлера с кожей замороженное', 'Замороженная мясная продукция'),
	(15, 'Филе бедра цыпленка-бройлера с кожей охлажденное', 'Охлажденная мясная продукция'),
	(16, 'Филе бедра цыпленка-бройлера с кожей замороженное', 'Замороженная мясная продукция'),
	(17, 'Филе грудки цыпленка-бройлера охлажденное', 'Охлажденная мясная продукция'),
	(18, 'Филе грудки цыпленка-бройлера замороженное', 'Замороженная мясная продукция'),
	(19, 'Филе бедра цыпленка-бройлера охлажденное', 'Охлажденная мясная продукция'),
	(20, 'Филе бедра цыпленка-бройлера замороженое', 'Замороженная мясная продукция'),
	(21, 'Печень цыпленка-бройлера охлажденная', 'Охлажденный субпродукт'),
	(22, 'Печень цыпленка-бройлера замороженная', 'Замороженный субпродукт'),
	(23, 'Желудки цыпленка-бройлера охлажденные', 'Охлажденный субпродукт'),
	(24, 'Желудки цыпленка-бройлера замороженные', 'Замороженный субпродукт'),
	(25, 'Сердце цыпленка-бройлера замороженное', 'Замороженный субпродукт'),
	(26, 'Сердце цыпленка-бройлера охлажденное', 'Охлажденный субпродукт'),
	(27, 'Суповой набор из цыпленка-бройлера (каркасы ц/б)', 'Замороженный полуфабрикат'),
	(28, 'Фарш куриный филейный охлажденный', 'Охлажденный полуфабрикат'),
	(29, 'Фарш куриный филейный замороженный', 'Замороженный полуфабрикат'),
	(30, 'Филе цыпленка-бройлера с кожей в маринаде охлажденное', 'Охлажденная мясная продукция'),
	(31, 'Филе цыпленка-бройлера с кожей в маринаде замороженное', 'Замороженная мясная продукция'),
	(32, 'Филе грудки цыпленка-бройлера в маринаде охлажденное', 'Охлажденная мясная продукция'),
	(33, 'Филе грудки цыпленка-бройлера в маринаде замороженное', 'Замороженная мясная продукция'),
	(34, 'Филе бедра цыпленка-бройлера в маринаде охлажденное', 'Охлажденная мясная продукция'),
	(35, 'Филе бедра цыпленка-бройлера в маринаде замороженное', 'Замороженная мясная продукция'),
	(36, 'Крылья цыпленка-бройлера в маринаде охлажденные', 'Охлажденная мясная продукция'),
	(37, 'Крылья цыпленка-бройлера в маринаде замороженные', 'Замороженная мясная продукция'),
	(38, 'Бедро цыпленка-бройлера в маринаде охлажденное', 'Охлажденная мясная продукция'),
	(39, 'Бедро цыпленка-бройлера в маринаде замороженное', 'Замороженная мясная продукция'),
	(40, 'Голень цыпленка-бройлера в маринаде замороженная', 'Замороженная мясная продукция'),
	(41, 'Голень цыпленка-бройлера в маринаде охлажденная', 'Охлажденная мясная продукция'),
	(42, 'Грудка ц/б варено-копченая охлажденная', 'Охлажденная мясная продукция'),
	(43, 'Филе цыпленка-бройлера с кожей охлажденное', 'Охлажденная мясная продукция'),
	(44, 'Филе цыпленка-бройлера с кожей замороженное', 'Замороженная мясная продукция'),
	(45, 'Филе цыпленка-бройлера без кожи охлажденное', 'Охлажденная мясная продукция'),
	(46, 'Филе цыпленка-бройлера без кожи замороженное', 'Замороженная мясная продукция'),
	(47, 'Яйцо куриное пищевое С0', 'Яичная продукция'),
	(48, 'Яйцо куриное пищевое С1', 'Яичная продукция'),
	(49, 'Яйцо куриное пищевое С1 мытое дезинфицированное', 'Яичная продукция'),
	(50, 'Яйцо куриное пищевое С0 мытое дезинфицированное', 'Яичная продукция'),
	(51, 'Крылья цыпленка-бройлера охлажденные 2ф', 'Охлажденная мясная продукция'),
	(52, 'Крылья цыпленка-бройлера замороженные 2ф', 'Замороженная мясная продукция'),
	(53, 'Тушка цыпленка-бройлера 1-го сорта охлажденная (Халяль)', 'Охлажденная мясная продукция'),
	(54, 'Тушка цыпленка-бройлера 1-го сорта замороженная (Халяль)', 'Замороженная мясная продукция'),
	(55, 'Обрезь филе ц/б охлажденная', 'Охлажденный полуфабрикат'),
	(56, 'Обрезь филе ц/б замороженная', 'Замороженный полуфабрикат'),
	(57, 'Кожа цыпленка-бройлера', 'Замороженный полуфабрикат'),
	(58, 'Суповой набор из цыпленка-бройлера (кости ц/б)', 'Замороженный полуфабрикат');

-- Insert missing root groups. Groups never have a measure unit.
WITH group_src AS (
	SELECT
		group_name,
		min(sort_order) AS sort_order
	FROM migration_product_seed
	GROUP BY group_name
)
INSERT INTO products (
	parent_id,
	name,
	sort_order,
	measure_unit_id,
	is_group,
	is_active
)
SELECT
	NULL,
	gs.group_name,
	gs.sort_order,
	NULL,
	true,
	true
FROM group_src gs
WHERE NOT EXISTS (
	SELECT 1
	FROM products p
	WHERE p.parent_id IS NULL
		AND p.is_group = true
		AND lower(btrim(p.name)) = lower(btrim(gs.group_name))
);

-- Also clear an accidentally assigned unit on an existing seeded group.
UPDATE products p
SET measure_unit_id = NULL
FROM (
	SELECT DISTINCT group_name
	FROM migration_product_seed
) gs
WHERE p.parent_id IS NULL
	AND p.is_group = true
	AND lower(btrim(p.name)) = lower(btrim(gs.group_name))
	AND p.measure_unit_id IS NOT NULL;

-- Existing products are matched globally by normalized name. Only their unit
-- is updated; their parent, name, sort order, and activity are preserved.
WITH product_src AS (
	SELECT
		s.product_name,
		CASE
			WHEN s.group_name = 'Яичная продукция' THEN '8751'
			ELSE '166'
		END AS okei_code
	FROM migration_product_seed s
)
UPDATE products p
SET measure_unit_id = mu.id
FROM product_src s
JOIN measure_units mu
	ON mu.okei_code = s.okei_code
WHERE p.is_group = false
	AND lower(btrim(p.name)) = lower(btrim(s.product_name))
	AND p.measure_unit_id IS DISTINCT FROM mu.id;

-- Insert products that do not yet exist. Meat uses kilograms (166), while
-- products in the egg group use boxes (8751).
WITH product_src AS (
	SELECT
		s.sort_order,
		s.product_name,
		s.group_name,
		CASE
			WHEN s.group_name = 'Яичная продукция' THEN '8751'
			ELSE '166'
		END AS okei_code
	FROM migration_product_seed s
),
groups AS (
	SELECT DISTINCT ON (lower(btrim(p.name)))
		lower(btrim(p.name)) AS normalized_name,
		p.id
	FROM products p
	WHERE p.parent_id IS NULL
		AND p.is_group = true
	ORDER BY lower(btrim(p.name)), p.id
)
INSERT INTO products (
	parent_id,
	name,
	sort_order,
	measure_unit_id,
	is_group,
	is_active
)
SELECT
	g.id,
	s.product_name,
	s.sort_order,
	mu.id,
	false,
	true
FROM product_src s
JOIN groups g
	ON g.normalized_name = lower(btrim(s.group_name))
JOIN measure_units mu
	ON mu.okei_code = s.okei_code
WHERE NOT EXISTS (
	SELECT 1
	FROM products p
	WHERE p.is_group = false
		AND lower(btrim(p.name)) = lower(btrim(s.product_name))
);

COMMIT;
