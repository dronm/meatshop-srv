-- Keep the shared JSON value compatible with webapp/constants while enforcing
-- the customer reference transactionally, including concurrent deletes/writes.
ALTER TABLE public.webapp_constants
	ADD COLUMN cash_customer_id integer GENERATED ALWAYS AS (
		CASE WHEN code = 'cash_customer' THEN (value #>> '{keys,id}')::integer ELSE NULL END
	) STORED,
	ADD CONSTRAINT webapp_constants_cash_customer_fk
		FOREIGN KEY (cash_customer_id) REFERENCES public.customers(id)
		ON UPDATE RESTRICT ON DELETE RESTRICT;
