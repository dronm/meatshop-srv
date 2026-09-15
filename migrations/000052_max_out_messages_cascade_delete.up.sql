BEGIN;

ALTER TABLE public.max_out_messages
	DROP CONSTRAINT max_out_messages_max_user_id_fkey;

ALTER TABLE public.max_out_messages
	ADD CONSTRAINT max_out_messages_max_user_id_fkey
	FOREIGN KEY (max_user_id)
	REFERENCES public.max_users(max_user_id)
	ON DELETE CASCADE
	ON UPDATE CASCADE;

COMMIT;
