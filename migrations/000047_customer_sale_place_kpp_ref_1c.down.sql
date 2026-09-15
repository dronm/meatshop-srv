BEGIN;

ALTER TABLE customer_sale_places DROP COLUMN IF EXISTS kpp;
ALTER TABLE customer_sale_places DROP COLUMN IF EXISTS ref_1c;

COMMIT;

