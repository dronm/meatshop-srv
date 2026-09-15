BEGIN;
-- Add trigger to specific table

-- users
DROP TRIGGER IF EXISTS audit_log_users ON users;

-- customers
DROP TRIGGER IF EXISTS audit_log_customers ON customers;

-- max_users
DROP TRIGGER IF EXISTS audit_log_max_users ON max_users;

-- measure_units
DROP TRIGGER IF EXISTS audit_log_measure_units ON measure_units;

-- orders
DROP TRIGGER IF EXISTS audit_log_orders ON orders;

-- order_items
DROP TRIGGER IF EXISTS audit_log_order_items ON order_items;

-- order_statuses
DROP TRIGGER IF EXISTS audit_log_order_statuses ON order_statuses;

-- products
DROP TRIGGER IF EXISTS audit_log_products ON products;


-- customer_sale_places
DROP TRIGGER IF EXISTS audit_log_customer_sale_places ON customer_sale_places;

COMMIT;


