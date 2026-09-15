BEGIN;

COMMENT ON COLUMN public.max_users.customer_sale_place_id IS
	'Default customer sale place for new MAX orders. A MAX user may select another active sale place of the same customer for each order.';

COMMIT;
