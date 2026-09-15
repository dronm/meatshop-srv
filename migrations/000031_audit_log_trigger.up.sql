BEGIN;
-- Add trigger to specific table

-- users
DROP TRIGGER IF EXISTS audit_log_users ON users;
CREATE TRIGGER audit_log_users
AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- customers
DROP TRIGGER IF EXISTS audit_log_customers ON customers;
CREATE TRIGGER audit_log_customers
AFTER INSERT OR UPDATE OR DELETE ON customers
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- customer_sale_places
DROP TRIGGER IF EXISTS audit_log_customer_sale_places ON customer_sale_places;
CREATE TRIGGER audit_log_customer_sale_places
AFTER INSERT OR UPDATE OR DELETE ON customer_sale_places
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- max_users
DROP TRIGGER IF EXISTS audit_log_max_users ON max_users;
CREATE TRIGGER audit_log_max_users
AFTER INSERT OR UPDATE OR DELETE ON max_users
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- measure_units
DROP TRIGGER IF EXISTS audit_log_measure_units ON measure_units;
CREATE TRIGGER audit_log_measure_units
AFTER INSERT OR UPDATE OR DELETE ON measure_units
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- orders
DROP TRIGGER IF EXISTS audit_log_orders ON orders;
CREATE TRIGGER audit_log_orders
AFTER INSERT OR UPDATE OR DELETE ON orders
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- order_items
DROP TRIGGER IF EXISTS audit_log_order_items ON order_items;
CREATE TRIGGER audit_log_order_items
AFTER INSERT OR UPDATE OR DELETE ON order_items
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- order_statuses
DROP TRIGGER IF EXISTS audit_log_order_statuses ON order_statuses;
CREATE TRIGGER audit_log_order_statuses
AFTER INSERT OR UPDATE OR DELETE ON order_statuses
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- products
DROP TRIGGER IF EXISTS audit_log_products ON products;
CREATE TRIGGER audit_log_products
AFTER INSERT OR UPDATE OR DELETE ON products
FOR EACH ROW EXECUTE FUNCTION audit_log_process();


COMMIT;

