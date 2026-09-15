BEGIN;

DROP FUNCTION IF EXISTS public.register_month_start_at(timestamptz);
DROP FUNCTION IF EXISTS public.register_month_start(timestamptz);
DROP FUNCTION IF EXISTS public.register_date_start(date);
DROP FUNCTION IF EXISTS public.register_business_timezone();

DROP TRIGGER IF EXISTS register_settings_validate_trigger ON public.register_settings;
DROP FUNCTION IF EXISTS public.register_settings_validate();
DROP TABLE IF EXISTS public.register_settings;

COMMIT;
