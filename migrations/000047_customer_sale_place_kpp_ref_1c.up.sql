BEGIN;

ALTER TABLE customer_sale_places ADD COLUMN IF NOT EXISTS kpp varchar(10);
ALTER TABLE customer_sale_places ADD COLUMN IF NOT EXISTS ref_1c varchar(36);

COMMIT;
