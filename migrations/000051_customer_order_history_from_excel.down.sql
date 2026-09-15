BEGIN;

CREATE TEMPORARY TABLE migration_order_history_rollback_ids (
	order_id integer PRIMARY KEY
) ON COMMIT DROP;

INSERT INTO migration_order_history_rollback_ids (order_id)
SELECT id
FROM public.orders
WHERE position(
	'[migration:000051_customer_order_history_from_excel;source_row='
	IN coalesce(comment_admin, '')
) = 1;

DO $migration$
DECLARE
	v_order_count integer;
BEGIN
	SELECT count(*) INTO v_order_count
	FROM migration_order_history_rollback_ids;

	IF v_order_count <> 84 THEN
		RAISE EXCEPTION
			'Expected 84 marked history orders for rollback; found %. The comment_admin migration marker must remain unchanged.',
			v_order_count;
	END IF;
END
$migration$;

-- Up does not create register actions, but remove any that may have been added
-- by a later application edit before deleting the imported documents.
DO $migration$
DECLARE
	v_order record;
BEGIN
	FOR v_order IN
		SELECT order_id
		FROM migration_order_history_rollback_ids
		ORDER BY order_id
	LOOP
		PERFORM public.ra_products_remove_acts('Order', v_order.order_id);
	END LOOP;
END
$migration$;

DELETE FROM public.orders AS orders
USING migration_order_history_rollback_ids AS rollback
WHERE orders.id = rollback.order_id;

COMMIT;
