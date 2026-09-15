BEGIN;

ALTER TABLE public.customer_sale_places
	DROP CONSTRAINT customer_sale_places_customer_id_fkey;

ALTER TABLE public.customer_sale_places
	ADD CONSTRAINT customer_sale_places_customer_id_fkey
	FOREIGN KEY (customer_id)
	REFERENCES public.customers(id)
	ON DELETE RESTRICT
	ON UPDATE CASCADE;

COMMIT;
