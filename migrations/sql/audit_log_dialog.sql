-- View: public.audit_log_dialog;

-- DROP VIEW public.audit_log_dialog;

CREATE OR REPLACE VIEW public.audit_log_dialog
 AS
SELECT 
	t.*,
	bt.changes

	FROM public.audit_log_list AS t
LEFT JOIN public.audit_log AS bt ON bt.id = t.id;



