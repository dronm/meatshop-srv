BEGIN;

DO $$
DECLARE
	v_customer_id integer := 1;
	v_sale_place_id integer;
	v_customer_user_id integer;

	v_status_new integer;
	v_status_processing integer;
	v_status_approved integer;
	v_status_completed integer;
	v_status_cancelled integer;

	v_order_new integer;
	v_order_processing integer;
	v_order_approved integer;
	v_order_completed integer;
	v_order_cancelled integer;

	v_product_count integer;
BEGIN
	-- Active sale place for customer.
	SELECT id
	INTO v_sale_place_id
	FROM customer_sale_places
	WHERE customer_id = v_customer_id
		AND is_active = TRUE
	ORDER BY id
	LIMIT 1;

	IF v_sale_place_id IS NULL THEN
		RAISE EXCEPTION
			'No active customer_sale_place found for customer_id=%',
			v_customer_id;
	END IF;

	-- Customer MAX user is optional for the order.
	SELECT id
	INTO v_customer_user_id
	FROM max_users
	WHERE customer_id = v_customer_id
	ORDER BY id
	LIMIT 1;

	-- Resolve statuses by code rather than relying on their IDs.
	SELECT id INTO v_status_new
	FROM order_statuses
	WHERE code = 'new';

	SELECT id INTO v_status_processing
	FROM order_statuses
	WHERE code = 'processing';

	SELECT id INTO v_status_approved
	FROM order_statuses
	WHERE code = 'approved';

	SELECT id INTO v_status_completed
	FROM order_statuses
	WHERE code = 'completed';

	SELECT id INTO v_status_cancelled
	FROM order_statuses
	WHERE code = 'cancelled';

	IF v_status_new IS NULL
		OR v_status_processing IS NULL
		OR v_status_approved IS NULL
		OR v_status_completed IS NULL
		OR v_status_cancelled IS NULL
	THEN
		RAISE EXCEPTION 'Required order statuses are missing';
	END IF;

	-- We need usable products for order_items.
	SELECT count(*)
	INTO v_product_count
	FROM products
	WHERE is_active = TRUE
		AND is_group = FALSE
		AND measure_unit_id IS NOT NULL;

	IF v_product_count < 3 THEN
		RAISE EXCEPTION
			'At least 3 active non-group products with measure_unit_id are required; found %',
			v_product_count;
	END IF;


	-- =========================================================
	-- 1. NEW ORDER
	-- Editable from the MAX Mini App.
	-- =========================================================

	INSERT INTO orders (
		for_date,
		customer_id,
		customer_sale_place_id,
		customer_user_id,
		status_id,
		comment_customer,
		comment_admin
	)
	VALUES (
		CURRENT_DATE,
		v_customer_id,
		v_sale_place_id,
		v_customer_user_id,
		v_status_new,
		'Тестовый новый заказ',
		NULL
	)
	RETURNING id INTO v_order_new;

	WITH selected_products AS (
		SELECT
			p.id,
			p.measure_unit_id,
			row_number() OVER (ORDER BY p.id)::integer AS line_num
		FROM products AS p
		WHERE p.is_active = TRUE
			AND p.is_group = FALSE
			AND p.measure_unit_id IS NOT NULL
		ORDER BY p.id
		LIMIT 3
	)
	INSERT INTO order_items (
		line_num,
		order_id,
		product_id,
		measure_unit_id,
		quant_required,
		quant
	)
	SELECT
		line_num,
		v_order_new,
		id,
		measure_unit_id,
		(line_num * 2)::numeric,
		(line_num * 2)::numeric
	FROM selected_products;


	-- =========================================================
	-- 2. PROCESSING ORDER
	-- Includes adjusted quantities.
	-- =========================================================

	INSERT INTO orders (
		for_date,
		number_1c,
		ref_1c,
		customer_id,
		customer_sale_place_id,
		customer_user_id,
		status_id,
		comment_customer,
		comment_admin
	)
	VALUES (
		CURRENT_DATE + 1,
		'TEST-0002',
		jsonb_build_object(
			'id', '00000000-0000-0000-0000-000000000002',
			'descr', 'Счет на оплату TEST-0002'
		),
		v_customer_id,
		v_sale_place_id,
		v_customer_user_id,
		v_status_processing,
		'Заказ находится в обработке',
		'Тестовый заказ'
	)
	RETURNING id INTO v_order_processing;

	WITH selected_products AS (
		SELECT
			p.id,
			p.measure_unit_id,
			row_number() OVER (ORDER BY p.id)::integer AS line_num
		FROM products AS p
		WHERE p.is_active = TRUE
			AND p.is_group = FALSE
			AND p.measure_unit_id IS NOT NULL
		ORDER BY p.id
		LIMIT 3
	)
	INSERT INTO order_items (
		line_num,
		order_id,
		product_id,
		measure_unit_id,
		quant_required,
		quant
	)
	SELECT
		line_num,
		v_order_processing,
		id,
		measure_unit_id,
		CASE line_num
			WHEN 1 THEN 10
			WHEN 2 THEN 5
			ELSE 8
		END,
		CASE line_num
			WHEN 1 THEN 8
			WHEN 2 THEN 5
			ELSE 6
		END
	FROM selected_products;


	-- =========================================================
	-- 3. APPROVED ORDER
	-- =========================================================

	INSERT INTO orders (
		for_date,
		number_1c,
		ref_1c,
		customer_id,
		customer_sale_place_id,
		customer_user_id,
		status_id,
		comment_customer,
		comment_admin
	)
	VALUES (
		CURRENT_DATE + 2,
		'TEST-0003',
		jsonb_build_object(
			'id', '00000000-0000-0000-0000-000000000003',
			'descr', 'Счет на оплату TEST-0003'
		),
		v_customer_id,
		v_sale_place_id,
		v_customer_user_id,
		v_status_approved,
		'Согласованный тестовый заказ',
		NULL
	)
	RETURNING id INTO v_order_approved;

	WITH selected_products AS (
		SELECT
			p.id,
			p.measure_unit_id,
			row_number() OVER (ORDER BY p.id)::integer AS line_num
		FROM products AS p
		WHERE p.is_active = TRUE
			AND p.is_group = FALSE
			AND p.measure_unit_id IS NOT NULL
		ORDER BY p.id
		LIMIT 3
	)
	INSERT INTO order_items (
		line_num,
		order_id,
		product_id,
		measure_unit_id,
		quant_required,
		quant
	)
	SELECT
		line_num,
		v_order_approved,
		id,
		measure_unit_id,
		(line_num + 2)::numeric,
		(line_num + 2)::numeric
	FROM selected_products;


	-- =========================================================
	-- 4. COMPLETED ORDER
	-- =========================================================

	INSERT INTO orders (
		for_date,
		number_1c,
		ref_1c,
		customer_id,
		customer_sale_place_id,
		customer_user_id,
		status_id,
		comment_customer,
		comment_admin
	)
	VALUES (
		CURRENT_DATE - 5,
		'TEST-0004',
		jsonb_build_object(
			'id', '00000000-0000-0000-0000-000000000004',
			'descr', 'Счет на оплату TEST-0004'
		),
		v_customer_id,
		v_sale_place_id,
		v_customer_user_id,
		v_status_completed,
		'Выполненный тестовый заказ',
		NULL
	)
	RETURNING id INTO v_order_completed;

	WITH selected_products AS (
		SELECT
			p.id,
			p.measure_unit_id,
			row_number() OVER (ORDER BY p.id)::integer AS line_num
		FROM products AS p
		WHERE p.is_active = TRUE
			AND p.is_group = FALSE
			AND p.measure_unit_id IS NOT NULL
		ORDER BY p.id
		LIMIT 3
	)
	INSERT INTO order_items (
		line_num,
		order_id,
		product_id,
		measure_unit_id,
		quant_required,
		quant
	)
	SELECT
		line_num,
		v_order_completed,
		id,
		measure_unit_id,
		(line_num * 3)::numeric,
		(line_num * 3)::numeric
	FROM selected_products;


	-- =========================================================
	-- 5. CANCELLED ORDER
	-- =========================================================

	INSERT INTO orders (
		for_date,
		customer_id,
		customer_sale_place_id,
		customer_user_id,
		status_id,
		comment_customer,
		comment_admin
	)
	VALUES (
		CURRENT_DATE - 10,
		v_customer_id,
		v_sale_place_id,
		v_customer_user_id,
		v_status_cancelled,
		'Отмененный тестовый заказ',
		'Заказ добавлен SQL-скриптом для тестирования'
	)
	RETURNING id INTO v_order_cancelled;

	WITH selected_products AS (
		SELECT
			p.id,
			p.measure_unit_id,
			row_number() OVER (ORDER BY p.id)::integer AS line_num
		FROM products AS p
		WHERE p.is_active = TRUE
			AND p.is_group = FALSE
			AND p.measure_unit_id IS NOT NULL
		ORDER BY p.id
		LIMIT 2
	)
	INSERT INTO order_items (
		line_num,
		order_id,
		product_id,
		measure_unit_id,
		quant_required,
		quant
	)
	SELECT
		line_num,
		v_order_cancelled,
		id,
		measure_unit_id,
		(line_num * 4)::numeric,
		(line_num * 4)::numeric
	FROM selected_products;


	RAISE NOTICE
		'Created mock orders: new=%, processing=%, approved=%, completed=%, cancelled=%',
		v_order_new,
		v_order_processing,
		v_order_approved,
		v_order_completed,
		v_order_cancelled;
END
$$;

COMMIT;
